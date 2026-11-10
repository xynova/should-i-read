package hoststore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	deleteCartMaxItems     = 200
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
	if len(items) > deleteCartMaxItems {
		return sirerr.New(sirerr.CodeInvalid, op, "too many cart items").
			With("count", strconv.Itoa(len(items)))
	}
	cleaned := make([]DeleteCartItem, 0, len(items))
	seen := map[string]struct{}{}
	for _, in := range items {
		item, err := normalizeDeleteCartItem(op, in)
		if err != nil {
			return err
		}
		if _, ok := seen[item.ObjectHash]; ok {
			continue
		}
		seen[item.ObjectHash] = struct{}{}
		cleaned = append(cleaned, item)
	}
	batchGroup := ""
	for _, item := range cleaned {
		if item.GroupID != "" {
			batchGroup = item.GroupID
			break
		}
	}
	if batchGroup == "" {
		id, err := newDeleteCartGroupID()
		if err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "new group id")
		}
		batchGroup = id
	}
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
	}
	if err := tx.Commit(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "commit")
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
