package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type memCart struct {
	mu    sync.Mutex
	items map[string]hoststore.DeleteCartItem
}

func newMemCart() *memCart {
	return &memCart{items: map[string]hoststore.DeleteCartItem{}}
}

func (m *memCart) AddDeleteCart(ctx context.Context, items []hoststore.DeleteCartItem) error {
	if err := requireCartCtx(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		h := item.ObjectHash
		if h == "" {
			return sirerr.New(sirerr.CodeInvalid, "memCart.AddDeleteCart", "empty object_hash")
		}
		m.items[h] = item
	}
	return nil
}

func (m *memCart) ListDeleteCart(ctx context.Context) ([]hoststore.DeleteCartItem, error) {
	if err := requireCartCtx(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]hoststore.DeleteCartItem, 0, len(m.items))
	for _, item := range m.items {
		out = append(out, item)
	}
	return out, nil
}

func (m *memCart) RemoveDeleteCart(ctx context.Context, objectHash string) error {
	if err := requireCartCtx(ctx); err != nil {
		return err
	}
	if objectHash == "" {
		return sirerr.New(sirerr.CodeInvalid, "memCart.RemoveDeleteCart", "empty object_hash")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, objectHash)
	return nil
}

func (m *memCart) RemoveDeleteCartGroup(ctx context.Context, groupID string) error {
	if err := requireCartCtx(ctx); err != nil {
		return err
	}
	if groupID == "" {
		return sirerr.New(sirerr.CodeInvalid, "memCart.RemoveDeleteCartGroup", "empty group_id")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for h, item := range m.items {
		if item.GroupID == groupID {
			delete(m.items, h)
		}
	}
	return nil
}

func requireCartCtx(ctx context.Context) error {
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, "memCart", "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return sirerr.New(sirerr.CodeInvalid, "memCart", "missing context deadline")
	}
	return nil
}

func TestCartAddListRemove(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: newMemCart()}).Handler())
	t.Cleanup(ts.Close)

	body, err := json.Marshal(cartAddRequest{
		TLDR: "Follow-up mail.",
		Items: []cartJSONItem{
			{ObjectHash: "h1", Sender: "Ann", Subject: "Meet", Date: "2026-10-08"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	pres, err := http.Post(ts.URL+"/cart", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer pres.Body.Close()
	if pres.StatusCode != http.StatusOK {
		t.Fatalf("post status %d", pres.StatusCode)
	}
	var added cartResponse
	if err := json.NewDecoder(pres.Body).Decode(&added); err != nil {
		t.Fatal(err)
	}
	if !added.OK || added.Count != 1 || added.Items[0].TLDR != "Follow-up mail." {
		t.Fatalf("added %+v", added)
	}

	gres, err := http.Get(ts.URL + "/cart")
	if err != nil {
		t.Fatal(err)
	}
	defer gres.Body.Close()
	var listed cartResponse
	if err := json.NewDecoder(gres.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if listed.Count != 1 {
		t.Fatalf("list %+v", listed)
	}

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/cart?hash=h1", nil)
	if err != nil {
		t.Fatal(err)
	}
	dres, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer dres.Body.Close()
	var removed cartResponse
	if err := json.NewDecoder(dres.Body).Decode(&removed); err != nil {
		t.Fatal(err)
	}
	if !removed.OK || removed.Count != 0 {
		t.Fatalf("removed %+v", removed)
	}
}

func TestCartRemoveGroup(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Cart: newMemCart()}).Handler())
	t.Cleanup(ts.Close)

	body, err := json.Marshal(cartAddRequest{
		GroupLabel: "Pipelines",
		Items: []cartJSONItem{
			{ObjectHash: "a1", Sender: "GitLab", Subject: "fail 1", GroupID: "g1"},
			{ObjectHash: "a2", Sender: "GitLab", Subject: "fail 2", GroupID: "g1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	pres, err := http.Post(ts.URL+"/cart", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer pres.Body.Close()
	if pres.StatusCode != http.StatusOK {
		t.Fatalf("post status %d", pres.StatusCode)
	}
	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/cart?group=g1", nil)
	if err != nil {
		t.Fatal(err)
	}
	dres, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer dres.Body.Close()
	var removed cartResponse
	if err := json.NewDecoder(dres.Body).Decode(&removed); err != nil {
		t.Fatal(err)
	}
	if !removed.OK || removed.Count != 0 {
		t.Fatalf("removed group %+v", removed)
	}
}

func TestCartUnavailableWithoutStore(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer((&IntentPageServer{HTML: []byte("h")}).Handler())
	t.Cleanup(ts.Close)
	res, err := http.Get(ts.URL + "/cart")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
}
