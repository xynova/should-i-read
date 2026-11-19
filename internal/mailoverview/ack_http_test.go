package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type memAckCart struct {
	mu    sync.Mutex
	items map[string]hoststore.AckCartItem
}

func newMemAckCart() *memAckCart {
	return &memAckCart{items: map[string]hoststore.AckCartItem{}}
}

func (m *memAckCart) AddAckCart(ctx context.Context, items []hoststore.AckCartItem) error {
	if err := requireCartCtx(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		h := item.ObjectHash
		if h == "" {
			return sirerr.New(sirerr.CodeInvalid, "memAckCart.AddAckCart", "empty object_hash")
		}
		m.items[h] = item
	}
	return nil
}

func (m *memAckCart) ListAckCart(ctx context.Context) ([]hoststore.AckCartItem, error) {
	if err := requireCartCtx(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]hoststore.AckCartItem, 0, len(m.items))
	for _, item := range m.items {
		out = append(out, item)
	}
	return out, nil
}

func (m *memAckCart) RemoveAckCart(ctx context.Context, objectHash string) error {
	if err := requireCartCtx(ctx); err != nil {
		return err
	}
	if objectHash == "" {
		return sirerr.New(sirerr.CodeInvalid, "memAckCart.RemoveAckCart", "empty object_hash")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, objectHash)
	return nil
}

func TestAckAddSkipsInCartAndActioned(t *testing.T) {
	t.Parallel()
	cart := newMemAckCart()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := cart.AddAckCart(ctx, []hoststore.AckCartItem{{ObjectHash: "already", Sender: "a"}}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer((&IntentPageServer{
		HTML:    []byte("h"),
		AckCart: cart,
		Actioned: stubActioned{m: map[string]maildelete.ActionedInfo{
			"trash": {Outcome: maildelete.OutcomeAlreadyInTrash},
		}},
	}).Handler())
	t.Cleanup(ts.Close)
	body, err := json.Marshal(ackAddRequest{Items: []ackJSONItem{
		{ObjectHash: "already", Sender: "a"},
		{ObjectHash: "trash", Sender: "github"},
		{ObjectHash: "live", Sender: "b"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/ack", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var out ackResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Count != 2 || out.SkippedInCart != 1 || out.SkippedActioned != 1 {
		t.Fatalf("%+v", out)
	}
}

func TestAckAddDropsDeleteCart(t *testing.T) {
	t.Parallel()
	cart := newMemCart()
	ack := newMemAckCart()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := cart.AddDeleteCart(ctx, []hoststore.DeleteCartItem{{ObjectHash: "switch", Sender: "stan"}}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer((&IntentPageServer{
		HTML:    []byte("h"),
		Cart:    cart,
		AckCart: ack,
	}).Handler())
	t.Cleanup(ts.Close)
	body, err := json.Marshal(ackAddRequest{Items: []ackJSONItem{
		{ObjectHash: "switch", Sender: "stan"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/ack", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	got, err := cart.ListDeleteCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("delete cart still has %+v", got)
	}
	listed, err := ack.ListAckCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ObjectHash != "switch" {
		t.Fatalf("ack cart %+v", listed)
	}
}

func TestAckCheckoutRejectsWrongConfirm(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer((&IntentPageServer{
		HTML:    []byte("h"),
		AckCart: newMemAckCart(),
		AckJobs: nil,
	}).Handler())
	t.Cleanup(ts.Close)
	body, err := json.Marshal(cartJobApplyRequest{Confirm: "seen-at-source"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(ts.URL+"/ack/checkout", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
}
