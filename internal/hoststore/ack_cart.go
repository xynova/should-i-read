package hoststore

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const ackCartAddMax = 200

// AckCartItem is one staged mark-read row. The mailbox is unchanged until apply.
type AckCartItem struct {
	ObjectHash string
	Sender     string
	Subject    string
	MailedAt   string
	AddedAt    string
}

// AddAckCart upserts staged Acked rows. It does not mark mail read.
func (s *Store) AddAckCart(ctx context.Context, items []AckCartItem) error {
	const op = "hoststore.Store.AddAckCart"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	if len(items) > ackCartAddMax {
		return sirerr.New(sirerr.CodeInvalid, op, "too many ack cart items").
			With("count", strconv.Itoa(len(items)))
	}
	cleaned := make([]AckCartItem, 0, len(items))
	seen := map[string]struct{}{}
	for _, in := range items {
		item, err := normalizeAckCartItem(op, in)
		if err != nil {
			return err
		}
		if _, ok := seen[item.ObjectHash]; ok {
			continue
		}
		seen[item.ObjectHash] = struct{}{}
		cleaned = append(cleaned, item)
	}
	if err := s.guardAckCartTotal(ctx, op, cleaned); err != nil {
		return err
	}
	stamp := s.stamp()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "begin tx")
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range cleaned {
		_, err := tx.ExecContext(ctx, `
INSERT INTO ack_cart (object_hash, sender, subject, mailed_at, added_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(object_hash) DO UPDATE SET
  sender=excluded.sender,
  subject=excluded.subject,
  mailed_at=excluded.mailed_at`,
			item.ObjectHash, item.Sender, item.Subject, item.MailedAt, stamp)
		if err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert ack cart row").
				With("object_hash", item.ObjectHash)
		}
	}
	if err := tx.Commit(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "commit")
	}
	return nil
}

// ListAckCart returns staged Acked rows, newest first.
func (s *Store) ListAckCart(ctx context.Context) ([]AckCartItem, error) {
	const op = "hoststore.Store.ListAckCart"
	if err := s.requireStore(ctx, op); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT object_hash, sender, subject, mailed_at, added_at
FROM ack_cart
ORDER BY added_at DESC, object_hash ASC`)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "query ack cart")
	}
	defer func() { _ = rows.Close() }()
	out := []AckCartItem{}
	for rows.Next() {
		var item AckCartItem
		if err := rows.Scan(&item.ObjectHash, &item.Sender, &item.Subject, &item.MailedAt, &item.AddedAt); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "scan ack cart")
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate ack cart")
	}
	return out, nil
}

// RemoveAckCart drops one staged Acked row.
func (s *Store) RemoveAckCart(ctx context.Context, objectHash string) error {
	const op = "hoststore.Store.RemoveAckCart"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	hash := strings.TrimSpace(objectHash)
	if hash == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM ack_cart WHERE object_hash=?`, hash); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "delete ack cart row").With("object_hash", hash)
	}
	return nil
}

// ClearAckCart drops every staged Acked row.
func (s *Store) ClearAckCart(ctx context.Context) error {
	const op = "hoststore.Store.ClearAckCart"
	if err := s.requireStore(ctx, op); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM ack_cart`); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "clear ack cart")
	}
	return nil
}

func normalizeAckCartItem(op string, in AckCartItem) (AckCartItem, error) {
	hash := strings.TrimSpace(in.ObjectHash)
	if hash == "" {
		return AckCartItem{}, sirerr.New(sirerr.CodeInvalid, op, "empty object_hash")
	}
	if utf8.RuneCountInString(hash) > deleteCartHashMax {
		return AckCartItem{}, sirerr.New(sirerr.CodeInvalid, op, "object_hash too long")
	}
	return AckCartItem{
		ObjectHash: hash,
		Sender:     clipRunes(strings.TrimSpace(in.Sender), deleteCartFieldMax),
		Subject:    clipRunes(strings.TrimSpace(in.Subject), deleteCartFieldMax),
		MailedAt:   clipRunes(strings.TrimSpace(in.MailedAt), deleteCartFieldMax),
	}, nil
}

func (s *Store) guardAckCartTotal(ctx context.Context, op string, incoming []AckCartItem) error {
	rows, err := s.db.QueryContext(ctx, `SELECT object_hash FROM ack_cart`)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "count ack cart")
	}
	defer func() { _ = rows.Close() }()
	have := map[string]struct{}{}
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "scan ack cart hash")
		}
		have[hash] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "iterate ack cart hashes")
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
		return sirerr.New(sirerr.CodeInvalid, op, "ack cart is full").
			With("count", strconv.Itoa(total)).
			With("max", strconv.Itoa(deleteCartMaxTotal))
	}
	return nil
}
