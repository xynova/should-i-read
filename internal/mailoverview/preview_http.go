package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const previewMaxBody = 8192

// MessagePreview is one message for the Reply/Decide panel.
type MessagePreview struct {
	Subject   string
	From      string
	Date      string
	Body      string
	Truncated bool
}

// MailPreviewer loads a message body for overview serve.
type MailPreviewer interface {
	MessagePreview(ctx context.Context, hash string) (MessagePreview, error)
}

type previewResponse struct {
	OK        bool   `json:"ok"`
	Subject   string `json:"subject,omitempty"`
	From      string `json:"from,omitempty"`
	Date      string `json:"date,omitempty"`
	Body      string `json:"body,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
	Kept      bool   `json:"kept,omitempty"`
	Error     string `json:"error,omitempty"`
}

// PimdirPreview loads RFC822 previews from a pimdir store.
type PimdirPreview struct {
	Reader *pimdir.Reader
}

// MessagePreview reads headers and a capped plain-text body.
func (p PimdirPreview) MessagePreview(ctx context.Context, hash string) (MessagePreview, error) {
	const op = "mailoverview.PimdirPreview.MessagePreview"
	if ctx == nil {
		return MessagePreview{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if p.Reader == nil {
		return MessagePreview{}, sirerr.New(sirerr.CodeUnavailable, op, "mail store is not open")
	}
	sum, err := p.Reader.FindByObjectHash(hash)
	if err != nil {
		return MessagePreview{}, err
	}
	blob, err := p.Reader.ReadBlob(hash)
	if err != nil {
		return MessagePreview{}, err
	}
	prev, err := pimdir.DecodeMessagePreview(blob, previewMaxBody)
	if err != nil {
		return MessagePreview{}, err
	}
	from := strings.TrimSpace(sum.SenderName)
	if from == "" {
		from = strings.TrimSpace(sum.Sender)
	} else if strings.TrimSpace(sum.Sender) != "" {
		from = from + " <" + strings.TrimSpace(sum.Sender) + ">"
	}
	return MessagePreview{
		Subject:   strings.TrimSpace(sum.Subject),
		From:      from,
		Date:      strings.TrimSpace(sum.Date),
		Body:      prev.Body,
		Truncated: prev.Truncated,
	}, nil
}

func (s *previewHTTP) servePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writePreviewJSON(w, http.StatusMethodNotAllowed, previewResponse{Error: httpErrGETRequired})
		return
	}
	if s == nil || s.Preview == nil {
		writePreviewJSON(w, http.StatusServiceUnavailable, previewResponse{Error: "message preview needs mail overview serve with a mail store"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writePreviewJSON(w, http.StatusInternalServerError, previewResponse{Error: httpErrNilRequestContext})
		return
	}
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if hash == "" {
		writePreviewJSON(w, http.StatusBadRequest, previewResponse{Error: "missing hash"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()
	prev, err := s.Preview.MessagePreview(ctx, hash)
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
		writePreviewJSON(w, status, previewResponse{Error: strings.TrimSpace(err.Error())})
		return
	}
	kept := false
	if s.Outcomes != nil {
		if o, ok, gerr := s.Outcomes.GetOutcome(ctx, hash); gerr == nil && ok && strings.TrimSpace(o.Outcome) == hoststore.OutcomeKept {
			kept = true
		}
	}
	writePreviewJSON(w, http.StatusOK, previewResponse{
		OK: true, Subject: prev.Subject, From: prev.From, Date: prev.Date,
		Body: prev.Body, Truncated: prev.Truncated, Kept: kept,
	})
}

func writePreviewJSON(w http.ResponseWriter, status int, body previewResponse) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode preview reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
