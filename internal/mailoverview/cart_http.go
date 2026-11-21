package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
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
	OK              bool           `json:"ok"`
	Items           []cartJSONItem `json:"items,omitempty"`
	Count           int            `json:"count"`
	SkippedInCart   int            `json:"skipped_in_cart,omitempty"`
	SkippedActioned int            `json:"skipped_actioned,omitempty"`
	Error           string         `json:"error,omitempty"`
}

func (s *cartHTTP) serveCart(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.Cart == nil {
		writeCartJSON(w, http.StatusServiceUnavailable, cartResponse{Error: "deletion cart needs mail overview serve"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeCartJSON(w, http.StatusInternalServerError, cartResponse{Error: httpErrNilRequestContext})
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

func (s *cartHTTP) listCart(ctx context.Context, w http.ResponseWriter) {
	s.writeCartList(ctx, w, 0, 0)
}

func (s *cartHTTP) writeCartList(ctx context.Context, w http.ResponseWriter, skippedInCart, skippedActioned int) {
	items, err := s.Cart.ListDeleteCart(ctx)
	if err != nil {
		writeCartErr(w, err)
		return
	}
	out := make([]cartJSONItem, 0, len(items))
	for _, item := range items {
		out = append(out, cartItemJSON(item))
	}
	writeCartJSON(w, http.StatusOK, cartResponse{
		OK: true, Items: out, Count: len(out),
		SkippedInCart: skippedInCart, SkippedActioned: skippedActioned,
	})
}

func (s *cartHTTP) addCart(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var err error
	ctx, span := startCartAddSpan(ctx)
	defer func() { endOverviewSpan(span, err) }()
	r.Body = http.MaxBytesReader(w, r.Body, cartMaxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeCartJSON(w, http.StatusBadRequest, cartResponse{Error: "could not read cart items"})
		return
	}
	var in cartAddRequest
	if err = json.Unmarshal(raw, &in); err != nil {
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
	var existing []hoststore.DeleteCartItem
	existing, err = s.Cart.ListDeleteCart(ctx)
	if err != nil {
		writeCartErr(w, err)
		return
	}
	actioned := map[string]maildelete.ActionedInfo{}
	lookup := s.actionedLookup()
	if lookup != nil {
		hashes := make([]string, 0, len(items))
		for _, item := range items {
			hashes = append(hashes, item.ObjectHash)
		}
		var got map[string]maildelete.ActionedInfo
		got, err = lookup.LookupActioned(ctx, hashes)
		if err != nil {
			writeCartErr(w, err)
			return
		}
		actioned = got
	}
	toAdd, skippedInCart, skippedActioned := filterCartAdds(existing, items, actioned)
	var movedFromAck int
	movedFromAck, err = s.dropAckCartHashes(ctx, objectHashesOfDelete(toAdd))
	if err != nil {
		writeCartErr(w, err)
		return
	}
	setCartAddCounts(span, len(items), len(toAdd), movedFromAck)
	if len(toAdd) > 0 {
		if err = s.Cart.AddDeleteCart(ctx, toAdd); err != nil {
			writeCartErr(w, err)
			return
		}
	}
	s.writeCartList(ctx, w, skippedInCart, skippedActioned)
}

func (s *cartHTTP) removeCart(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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

func cartClientError(err error) string {
	var se *sirerr.Error
	if errors.As(err, &se) && se != nil && strings.TrimSpace(se.Message) != "" {
		switch se.Message {
		case "too many cart items for job":
			n := se.Fields["count"]
			if n == "" {
				return "The deletion cart has too many messages for one Trash job. Remove some folders, then try Apply at source again."
			}
			return "The deletion cart has " + n + " messages, too many for one Trash job. Remove some folders, then try Apply at source again."
		case "too many cart items", "deletion cart is full":
			return "The deletion cart is full. Remove some folders, then mark fewer messages."
		case "empty deletion cart", "cart is empty":
			return "The deletion cart is empty."
		default:
			return strings.TrimSpace(se.Message)
		}
	}
	return strings.TrimSpace(err.Error())
}

func writeCartErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeInvalid {
		status = http.StatusBadRequest
	}
	writeCartJSON(w, status, cartResponse{Error: cartClientError(err)})
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
