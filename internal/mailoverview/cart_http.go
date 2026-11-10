package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const cartHTTPTimeout = intentHTTPTimeout

type cartJSONItem struct {
	ObjectHash string `json:"object_hash"`
	Sender     string `json:"sender"`
	Subject    string `json:"subject"`
	Date       string `json:"date"`
	TLDR       string `json:"tldr,omitempty"`
	GroupID    string `json:"group_id,omitempty"`
	GroupLabel string `json:"group_label,omitempty"`
	AddedAt    string `json:"added_at,omitempty"`
}

type cartAddRequest struct {
	Items      []cartJSONItem `json:"items"`
	TLDR       string         `json:"tldr"`
	GroupLabel string         `json:"group_label"`
}

type cartResponse struct {
	OK    bool           `json:"ok"`
	Items []cartJSONItem `json:"items,omitempty"`
	Count int            `json:"count"`
	Error string         `json:"error,omitempty"`
}

func (s *IntentPageServer) serveCart(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.Cart == nil {
		writeCartJSON(w, http.StatusServiceUnavailable, cartResponse{Error: "deletion cart needs mail overview serve"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeCartJSON(w, http.StatusInternalServerError, cartResponse{Error: "nil request context"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		s.listCart(ctx, w)
	case http.MethodPost:
		s.addCart(ctx, w, r)
	case http.MethodDelete:
		s.removeCart(ctx, w, r)
	default:
		writeCartJSON(w, http.StatusMethodNotAllowed, cartResponse{Error: "GET, POST, or DELETE required"})
	}
}

func (s *IntentPageServer) listCart(ctx context.Context, w http.ResponseWriter) {
	items, err := s.Cart.ListDeleteCart(ctx)
	if err != nil {
		writeCartErr(w, err)
		return
	}
	out := make([]cartJSONItem, 0, len(items))
	for _, item := range items {
		out = append(out, cartItemJSON(item))
	}
	writeCartJSON(w, http.StatusOK, cartResponse{OK: true, Items: out, Count: len(out)})
}

func (s *IntentPageServer) addCart(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, cartMaxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeCartJSON(w, http.StatusBadRequest, cartResponse{Error: "could not read cart items"})
		return
	}
	var in cartAddRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeCartJSON(w, http.StatusBadRequest, cartResponse{Error: "cart items must be JSON"})
		return
	}
	tldr := strings.TrimSpace(in.TLDR)
	label := strings.TrimSpace(in.GroupLabel)
	items := make([]hoststore.DeleteCartItem, 0, len(in.Items))
	for _, row := range in.Items {
		item := hoststore.DeleteCartItem{
			ObjectHash: row.ObjectHash,
			Sender:     row.Sender,
			Subject:    row.Subject,
			MailedAt:   row.Date,
			TLDR:       strings.TrimSpace(row.TLDR),
			GroupID:    strings.TrimSpace(row.GroupID),
			GroupLabel: strings.TrimSpace(row.GroupLabel),
		}
		if item.TLDR == "" {
			item.TLDR = tldr
		}
		if item.GroupLabel == "" {
			item.GroupLabel = label
		}
		items = append(items, item)
	}
	if err := s.Cart.AddDeleteCart(ctx, items); err != nil {
		writeCartErr(w, err)
		return
	}
	s.listCart(ctx, w)
}

func (s *IntentPageServer) removeCart(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	group := strings.TrimSpace(r.URL.Query().Get("group"))
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	var err error
	if group != "" {
		err = s.Cart.RemoveDeleteCartGroup(ctx, group)
	} else {
		err = s.Cart.RemoveDeleteCart(ctx, hash)
	}
	if err != nil {
		writeCartErr(w, err)
		return
	}
	s.listCart(ctx, w)
}

func cartItemJSON(item hoststore.DeleteCartItem) cartJSONItem {
	return cartJSONItem{
		ObjectHash: item.ObjectHash,
		Sender:     item.Sender,
		Subject:    item.Subject,
		Date:       item.MailedAt,
		TLDR:       item.TLDR,
		GroupID:    item.GroupID,
		GroupLabel: item.GroupLabel,
		AddedAt:    item.AddedAt,
	}
}

func writeCartErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeInvalid {
		status = http.StatusBadRequest
	}
	writeCartJSON(w, status, cartResponse{Error: strings.TrimSpace(err.Error())})
}

func writeCartJSON(w http.ResponseWriter, status int, body cartResponse) {
	if body.Items == nil && status == http.StatusOK {
		body.Items = []cartJSONItem{}
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode cart reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
