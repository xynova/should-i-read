package mailoverview

import (
	"context"
	"fmt"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
)

func (s *IntentPageServer) dropAckCartHashes(ctx context.Context, hashes []string) (int, error) {
	if s == nil || s.AckCart == nil || len(hashes) == 0 {
		return 0, nil
	}
	acked, err := s.AckCart.ListAckCart(ctx)
	if err != nil {
		return 0, fmt.Errorf("list ack cart: %w", err)
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
			return dropped, fmt.Errorf("drop ack cart %s: %w", h, err)
		}
		dropped++
		delete(have, h)
	}
	return dropped, nil
}

func (s *IntentPageServer) dropDeleteCartHashes(ctx context.Context, hashes []string) error {
	if s == nil || s.Cart == nil {
		return nil
	}
	for _, h := range hashes {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if err := s.Cart.RemoveDeleteCart(ctx, h); err != nil {
			return fmt.Errorf("drop delete cart %s: %w", h, err)
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
