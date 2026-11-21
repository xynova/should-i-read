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
	intentHTTPTimeout       = 45 * time.Second
	intentDetailHTTPTimeout = 90 * time.Second
	intentMaxBody           = 32 << 10
	cartMaxBody             = 256 << 10
)

// CartStore is the report-only deletion cart. It must not mutate the mailbox.
type CartStore interface {
	AddDeleteCart(ctx context.Context, items []hoststore.DeleteCartItem) error
	ListDeleteCart(ctx context.Context) ([]hoststore.DeleteCartItem, error)
	RemoveDeleteCart(ctx context.Context, objectHash string) error
	RemoveDeleteCartGroup(ctx context.Context, groupID string) error
}

// ActionedLookup classifies hashes that must leave the live months.
type ActionedLookup interface {
	LookupActioned(ctx context.Context, hashes []string) (map[string]maildelete.ActionedInfo, error)
}

// IntentPageServer composes page, cart, ack, and intent servers for overview mux.
type IntentPageServer struct {
	PageServer
	CartServer
	AckServer
	IntentServer
}

// ReplacePage swaps the served HTML and hash list after a post-sync report.
func (s *PageServer) ReplacePage(html []byte, hashes []string) {
	if s == nil {
		return
	}
	s.pageMu.Lock()
	s.HTML = append([]byte(nil), html...)
	s.Hashes = append([]string(nil), hashes...)
	s.pageMu.Unlock()
}

// Handler mounts page, cart, ack, and intent servers onto one mux.
func (s *IntentPageServer) Handler() http.Handler {
	mux := http.NewServeMux()
	s.mountHandlers(mux)
	return mux
}

func (s *CartServer) actionedLookup() ActionedLookup {
	if s == nil {
		return nil
	}
	if s.Actioned != nil {
		return s.Actioned
	}
	if s.Jobs != nil {
		return s.Jobs
	}
	return nil
}

func (s *pageHTTP) servePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	s.pageMu.RLock()
	html := s.HTML
	s.pageMu.RUnlock()
	_, _ = w.Write(html)
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

type intentDetailMessageJSON struct {
	Date         string  `json:"date,omitempty"`
	Sender       string  `json:"sender,omitempty"`
	Subject      string  `json:"subject,omitempty"`
	About        string  `json:"about,omitempty"`
	Snippet      string  `json:"snippet,omitempty"`
	NeedsReply   float64 `json:"needs_reply,omitempty"`
	Decision     float64 `json:"decision,omitempty"`
	Urgency      float64 `json:"urgency,omitempty"`
	OutlierFirst bool    `json:"outlier_first,omitempty"`
}

type intentDetailRequest struct {
	Month    string                    `json:"month"`
	Summary  string                    `json:"summary,omitempty"`
	Messages []intentDetailMessageJSON `json:"messages"`
}

type intentDetailResponse struct {
	OK     bool   `json:"ok"`
	Intent string `json:"intent,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (s *intentHTTP) serveIntent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeIntentJSON(w, http.StatusMethodNotAllowed, intentResponse{Error: httpErrPOSTRequired})
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
		writeIntentJSON(w, http.StatusInternalServerError, intentResponse{Error: httpErrNilRequestContext})
		return
	}
	ctx, cancel := context.WithTimeout(parent, intentHTTPTimeout)
	defer cancel()
	intent, err := s.Guess.Guess(ctx, IntentInput(in))
	if err != nil {
		status := http.StatusInternalServerError
		if c, ok := sirerr.AsCode(err); ok {
			switch c {
			case sirerr.CodeInvalid:
				status = http.StatusBadRequest
			case sirerr.CodeNotFound:
				status = http.StatusNotFound
			case sirerr.CodeUnavailable:
				status = http.StatusServiceUnavailable
			case sirerr.CodeFailed, sirerr.CodeAI, sirerr.CodeAuth, sirerr.CodeNetwork, sirerr.CodeNeedsReview:
				status = http.StatusInternalServerError
			}
		}
		writeIntentJSON(w, status, intentResponse{Error: strings.TrimSpace(err.Error())})
		return
	}
	writeIntentJSON(w, http.StatusOK, intentResponse{OK: true, Intent: intent})
}

func (s *intentHTTP) serveIntentDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeIntentDetailJSON(w, http.StatusMethodNotAllowed, intentDetailResponse{Error: httpErrPOSTRequired})
		return
	}
	if s == nil || s.Guess == nil {
		writeIntentDetailJSON(w, http.StatusServiceUnavailable, intentDetailResponse{Error: "intent guesser is not configured"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, intentMaxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeIntentDetailJSON(w, http.StatusBadRequest, intentDetailResponse{Error: "could not read selection"})
		return
	}
	var in intentDetailRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeIntentDetailJSON(w, http.StatusBadRequest, intentDetailResponse{Error: "selection must be JSON"})
		return
	}
	msgs := make([]IntentDetailMessage, 0, len(in.Messages))
	for _, m := range in.Messages {
		msgs = append(msgs, IntentDetailMessage(m))
	}
	parent := r.Context()
	if parent == nil {
		writeIntentDetailJSON(w, http.StatusInternalServerError, intentDetailResponse{Error: httpErrNilRequestContext})
		return
	}
	ctx, cancel := context.WithTimeout(parent, intentDetailHTTPTimeout)
	defer cancel()
	intent, err := s.Guess.GuessDetail(ctx, IntentDetailInput{
		Month:    in.Month,
		Summary:  in.Summary,
		Messages: msgs,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if c, ok := sirerr.AsCode(err); ok {
			switch c {
			case sirerr.CodeInvalid:
				status = http.StatusBadRequest
			case sirerr.CodeNotFound:
				status = http.StatusNotFound
			case sirerr.CodeUnavailable:
				status = http.StatusServiceUnavailable
			case sirerr.CodeFailed, sirerr.CodeAI, sirerr.CodeAuth, sirerr.CodeNetwork, sirerr.CodeNeedsReview:
				status = http.StatusInternalServerError
			}
		}
		writeIntentDetailJSON(w, status, intentDetailResponse{Error: strings.TrimSpace(err.Error())})
		return
	}
	writeIntentDetailJSON(w, http.StatusOK, intentDetailResponse{OK: true, Intent: intent})
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

func writeIntentDetailJSON(w http.ResponseWriter, status int, body intentDetailResponse) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode intent detail reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
