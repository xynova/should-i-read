package mailoverview

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func (s *AckServer) dropAckCartHashes(ctx context.Context, hashes []string) (int, error) {
	const op = "mailoverview.dropAckCartHashes"
	if s == nil || s.AckCart == nil || len(hashes) == 0 {
		return 0, nil
	}
	acked, err := s.AckCart.ListAckCart(ctx)
	if err != nil {
		return 0, sirerr.Wrap(err, sirerr.CodeFailed, op, "list ack cart")
	}
	have := map[string]struct{}{}
	for _, item := range acked {
		h := strings.TrimSpace(item.ObjectHash)
		if h != "" {
			have[h] = struct{}{}
		}
	}
	if len(have) == 0 {
		return 0, nil
	}
	dropped := 0
	for _, h := range hashes {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if _, ok := have[h]; !ok {
			continue
		}
		if err := s.AckCart.RemoveAckCart(ctx, h); err != nil {
			return dropped, sirerr.Wrap(err, sirerr.CodeFailed, op, "drop ack cart").With("object_hash", h)
		}
		dropped++
		delete(have, h)
	}
	return dropped, nil
}

func (s *CartServer) dropDeleteCartHashes(ctx context.Context, hashes []string) error {
	const op = "mailoverview.dropDeleteCartHashes"
	if s == nil || s.Cart == nil {
		return nil
	}
	for _, h := range hashes {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if err := s.Cart.RemoveDeleteCart(ctx, h); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "drop delete cart").With("object_hash", h)
		}
	}
	return nil
}

func objectHashesOfAck(items []hoststore.AckCartItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		h := strings.TrimSpace(item.ObjectHash)
		if h != "" {
			out = append(out, h)
		}
	}
	return out
}

func objectHashesOfDelete(items []hoststore.DeleteCartItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		h := strings.TrimSpace(item.ObjectHash)
		if h != "" {
			out = append(out, h)
		}
	}
	return out
}
