package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	intentHTTPTimeout = 45 * time.Second
	intentMaxBody     = 32 << 10
	cartMaxBody       = 256 << 10
)

// CartStore is the report-only deletion cart. It must not mutate the mailbox.
type CartStore interface {
	AddDeleteCart(ctx context.Context, items []hoststore.DeleteCartItem) error
	ListDeleteCart(ctx context.Context) ([]hoststore.DeleteCartItem, error)
	RemoveDeleteCart(ctx context.Context, objectHash string) error
	RemoveDeleteCartGroup(ctx context.Context, groupID string) error
}

// IntentPageServer serves overview HTML, POST /intent, and the deletion cart.
type IntentPageServer struct {
	HTML  []byte
	Guess *IntentGuesser
	Cart  CartStore
	Jobs  *maildelete.Service
}

// Handler returns the overview mux.
func (s *IntentPageServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.servePage)
	mux.HandleFunc("/intent", s.serveIntent)
	mux.HandleFunc("/cart", s.serveCart)
	mux.HandleFunc("/cart/jobs", s.serveCartJobs)
	mux.HandleFunc("/cart/jobs/", s.serveCartJobs)
	return mux
}

func (s *IntentPageServer) servePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(s.HTML)
}

type intentRequest struct {
	Month string `json:"month"`
	Text  string `json:"text"`
}

type intentResponse struct {
	OK     bool   `json:"ok"`
	Intent string `json:"intent,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *IntentPageServer) serveIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeIntentJSON(w, http.StatusMethodNotAllowed, intentResponse{Error: "POST required"})
		return
	}
	if s == nil || s.Guess == nil {
		writeIntentJSON(w, http.StatusServiceUnavailable, intentResponse{Error: "intent guesser is not configured"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, intentMaxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeIntentJSON(w, http.StatusBadRequest, intentResponse{Error: "could not read selection"})
		return
	}
	var in intentRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeIntentJSON(w, http.StatusBadRequest, intentResponse{Error: "selection must be JSON"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeIntentJSON(w, http.StatusInternalServerError, intentResponse{Error: "nil request context"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, intentHTTPTimeout)
	defer cancel()
	intent, err := s.Guess.Guess(ctx, IntentInput{Month: in.Month, Text: in.Text})
	if err != nil {
		status := http.StatusInternalServerError
		if c, ok := sirerr.AsCode(err); ok {
			switch c {
			case sirerr.CodeInvalid:
				status = http.StatusBadRequest
			case sirerr.CodeUnavailable:
				status = http.StatusServiceUnavailable
			}
		}
		writeIntentJSON(w, status, intentResponse{Error: strings.TrimSpace(err.Error())})
		return
	}
	writeIntentJSON(w, http.StatusOK, intentResponse{OK: true, Intent: intent})
}

func writeIntentJSON(w http.ResponseWriter, status int, body intentResponse) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode intent reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
