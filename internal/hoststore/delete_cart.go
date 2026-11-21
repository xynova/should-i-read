package hoststore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	deleteCartAddMax       = 200
	deleteCartMaxTotal     = 2000
	deleteCartHashMax      = 128
	deleteCartFieldMax     = 500
	deleteCartTLDRMax      = 2000
	deleteCartGroupIDMax   = 64
	deleteCartGroupIDBytes = 8
)

// DeleteCartItem is one report-only deletion-cart row. The mailbox is unchanged.
type DeleteCartItem struct {
	ObjectHash string
	Sender     string
	Subject    string
	MailedAt   string
	TLDR       string
	GroupID    string
	GroupLabel string
	AddedAt    string
}

// AddDeleteCart upserts report-only cart rows. It does not trash or move mail.
func (s *Store) AddDeleteCart(ctx context.Context, items []DeleteCartItem) error {
	const op = "hoststore.Store.AddDeleteCart"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	if len(items) > deleteCartAddMax {
		return sirerr.New(sirerr.CodeInvalid, op, "too many cart items").
			With("count", strconv.Itoa(len(items)))
	}
	cleaned, err := dedupeDeleteCartItems(op, items)
	if err != nil {
		return err
	}
	if err := s.guardDeleteCartTotal(ctx, op, cleaned); err != nil {
		return err
	}
	batchGroup, err := resolveDeleteCartBatchGroup(cleaned)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "new group id")
	}
	return s.upsertDeleteCartRows(ctx, op, cleaned, batchGroup)
}

func dedupeDeleteCartItems(op string, items []DeleteCartItem) ([]DeleteCartItem, error) {
	cleaned := make([]DeleteCartItem, 0, len(items))
	seen := map[string]struct{}{}
	for _, in := range items {
		item, err := normalizeDeleteCartItem(op, in)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[item.ObjectHash]; ok {
			continue
		}
		seen[item.ObjectHash] = struct{}{}
		cleaned = append(cleaned, item)
	}
	return cleaned, nil
}

func resolveDeleteCartBatchGroup(cleaned []DeleteCartItem) (string, error) {
	for _, item := range cleaned {
		if item.GroupID != "" {
			return item.GroupID, nil
		}
	}
	return newDeleteCartGroupID()
}

func (s *Store) upsertDeleteCartRows(ctx context.Context, op string, cleaned []DeleteCartItem, batchGroup string) error {
	stamp := s.stamp()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range cleaned {
		if item.GroupID == "" {
			item.GroupID = batchGroup
		}
		if err := execDeleteCartUpsert(ctx, tx, op, item, stamp); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "commit")
	}
	return nil
}

func execDeleteCartUpsert(ctx context.Context, tx *sql.Tx, op string, item DeleteCartItem, stamp string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO delete_cart (object_hash, sender, subject, mailed_at, tldr, group_id, group_label, added_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(object_hash) DO UPDATE SET
  sender=excluded.sender,
  subject=excluded.subject,
  mailed_at=excluded.mailed_at,
  tldr=excluded.tldr,
  group_id=excluded.group_id,
  group_label=excluded.group_label`,
		item.ObjectHash, item.Sender, item.Subject, item.MailedAt, item.TLDR, item.GroupID, item.GroupLabel, stamp)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert cart row").
			With("object_hash", item.ObjectHash)
	}
	return nil
}

// ListDeleteCart returns cart rows, newest first.
func (s *Store) ListDeleteCart(ctx context.Context) ([]DeleteCartItem, error) {
	const op = "hoststore.Store.ListDeleteCart"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT object_hash, sender, subject, mailed_at, tldr, group_id, group_label, added_at
FROM delete_cart
ORDER BY added_at DESC, object_hash ASC`)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query cart")
	}
	defer func() { _ = rows.Close() }()
	out := []DeleteCartItem{}
	for rows.Next() {
		var item DeleteCartItem
		if err := rows.Scan(&item.ObjectHash, &item.Sender, &item.Subject, &item.MailedAt, &item.TLDR, &item.GroupID, &item.GroupLabel, &item.AddedAt); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan cart")
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate cart")
	}
	return out, nil
}

// RemoveDeleteCart drops one cart row. It does not change the mailbox.
func (s *Store) RemoveDeleteCart(ctx context.Context, objectHash string) error {
	const op = "hoststore.Store.RemoveDeleteCart"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM delete_cart WHERE object_hash=?`, hash); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "delete cart row").With("object_hash", hash)
	}
	return nil
}

// RemoveDeleteCartGroup drops every cart row in one Mark click. It does not change the mailbox.
func (s *Store) RemoveDeleteCartGroup(ctx context.Context, groupID string) error {
	const op = "hoststore.Store.RemoveDeleteCartGroup"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	id := strings.TrimSpace(groupID)
	if id == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty group_id")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM delete_cart WHERE group_id=?`, id); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "delete cart group").With("group_id", id)
	}
	return nil
}

// ClearDeleteCart drops every cart row. It does not change the mailbox.
func (s *Store) ClearDeleteCart(ctx context.Context) error {
	const op = "hoststore.Store.ClearDeleteCart"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM delete_cart`); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "clear cart")
	}
	return nil
}

func normalizeDeleteCartItem(op string, in DeleteCartItem) (DeleteCartItem, error) {
	hash := strings.TrimSpace(in.ObjectHash)
	if hash == "" {
		return DeleteCartItem{}, sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	if utf8.RuneCountInString(hash) > deleteCartHashMax {
		return DeleteCartItem{}, sirerr.New(sirerr.CodeInvalid, op, "object_hash too long")
	}
	return DeleteCartItem{
		ObjectHash: hash,
		Sender:     clipRunes(strings.TrimSpace(in.Sender), deleteCartFieldMax),
		Subject:    clipRunes(strings.TrimSpace(in.Subject), deleteCartFieldMax),
		MailedAt:   clipRunes(strings.TrimSpace(in.MailedAt), deleteCartFieldMax),
		TLDR:       clipRunes(strings.TrimSpace(in.TLDR), deleteCartTLDRMax),
		GroupID:    clipRunes(strings.TrimSpace(in.GroupID), deleteCartGroupIDMax),
		GroupLabel: clipRunes(strings.TrimSpace(in.GroupLabel), deleteCartFieldMax),
	}, nil
}

func (s *Store) guardDeleteCartTotal(ctx context.Context, op string, incoming []DeleteCartItem) error {
	rows, err := s.db.QueryContext(ctx, `SELECT object_hash FROM delete_cart`)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "count cart")
	}
	defer func() { _ = rows.Close() }()
	have := map[string]struct{}{}
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "scan cart hash")
		}
		have[hash] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate cart hashes")
	}
	total := len(have)
	for _, item := range incoming {
		if _, ok := have[item.ObjectHash]; ok {
			continue
		}
		have[item.ObjectHash] = struct{}{}
		total++
	}
	if total > deleteCartMaxTotal {
		return sirerr.New(sirerr.CodeInvalid, op, "deletion cart is full").
			With("count", strconv.Itoa(total)).
			With("max", strconv.Itoa(deleteCartMaxTotal))
	}
	return nil
}

func newDeleteCartGroupID() (string, error) {
	var b [deleteCartGroupIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func clipRunes(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
