package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailack"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type ackJSONItem struct {
	ObjectHash string `json:"object_hash"`
	Sender     string `json:"sender"`
	Subject    string `json:"subject"`
	Date       string `json:"date"`
	AddedAt    string `json:"added_at,omitempty"`
}

type ackAddRequest struct {
	Items []ackJSONItem `json:"items"`
}

type ackResponse struct {
	OK              bool          `json:"ok"`
	Items           []ackJSONItem `json:"items,omitempty"`
	Count           int           `json:"count"`
	SkippedInCart   int           `json:"skipped_in_cart,omitempty"`
	SkippedActioned int           `json:"skipped_actioned,omitempty"`
	Error           string        `json:"error,omitempty"`
}

// AckCartStore is the staged Acked list. It must not mutate the mailbox.
type AckCartStore interface {
	AddAckCart(ctx context.Context, items []hoststore.AckCartItem) error
	ListAckCart(ctx context.Context) ([]hoststore.AckCartItem, error)
	RemoveAckCart(ctx context.Context, objectHash string) error
}

func (s *IntentPageServer) serveAck(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.AckCart == nil {
		writeAckJSON(w, http.StatusServiceUnavailable, ackResponse{Error: "ack cart needs mail overview serve"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeAckJSON(w, http.StatusInternalServerError, ackResponse{Error: "ack request needs a context"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		s.listAck(ctx, w)
	case http.MethodPost:
		s.addAck(ctx, w, r)
	case http.MethodDelete:
		s.removeAck(ctx, w, r)
	default:
		writeAckJSON(w, http.StatusMethodNotAllowed, ackResponse{Error: "use GET, POST, or DELETE"})
	}
}

func (s *IntentPageServer) listAck(ctx context.Context, w http.ResponseWriter) {
	s.writeAckList(ctx, w, 0, 0)
}

func (s *IntentPageServer) writeAckList(ctx context.Context, w http.ResponseWriter, skippedInCart, skippedActioned int) {
	items, err := s.AckCart.ListAckCart(ctx)
	if err != nil {
		writeAckErr(w, err)
		return
	}
	out := make([]ackJSONItem, 0, len(items))
	for _, item := range items {
		out = append(out, ackJSONItem{
			ObjectHash: item.ObjectHash,
			Sender:     item.Sender,
			Subject:    item.Subject,
			Date:       item.MailedAt,
			AddedAt:    item.AddedAt,
		})
	}
	writeAckJSON(w, http.StatusOK, ackResponse{
		OK: true, Items: out, Count: len(out),
		SkippedInCart: skippedInCart, SkippedActioned: skippedActioned,
	})
}

func (s *IntentPageServer) addAck(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, cartMaxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeAckJSON(w, http.StatusBadRequest, ackResponse{Error: "could not read ack items"})
		return
	}
	var in ackAddRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeAckJSON(w, http.StatusBadRequest, ackResponse{Error: "ack items must be JSON"})
		return
	}
	items := make([]hoststore.AckCartItem, 0, len(in.Items))
	for _, row := range in.Items {
		items = append(items, hoststore.AckCartItem{
			ObjectHash: row.ObjectHash,
			Sender:     row.Sender,
			Subject:    row.Subject,
			MailedAt:   row.Date,
		})
	}
	existing, err := s.AckCart.ListAckCart(ctx)
	if err != nil {
		writeAckErr(w, err)
		return
	}
	actioned := map[string]maildelete.ActionedInfo{}
	lookup := s.actionedLookup()
	if lookup != nil {
		hashes := make([]string, 0, len(items))
		for _, item := range items {
			hashes = append(hashes, item.ObjectHash)
		}
		got, aerr := lookup.LookupActioned(ctx, hashes)
		if aerr != nil {
			writeAckErr(w, aerr)
			return
		}
		actioned = got
	}
	have := map[string]struct{}{}
	for _, item := range existing {
		have[item.ObjectHash] = struct{}{}
	}
	toAdd := make([]hoststore.AckCartItem, 0, len(items))
	skippedInCart := 0
	skippedActioned := 0
	for _, item := range items {
		h := strings.TrimSpace(item.ObjectHash)
		if h == "" {
			continue
		}
		if _, ok := actioned[h]; ok {
			skippedActioned++
			continue
		}
		if _, ok := have[h]; ok {
			skippedInCart++
			continue
		}
		have[h] = struct{}{}
		toAdd = append(toAdd, item)
	}
	if err := s.dropDeleteCartHashes(ctx, objectHashesOfAck(toAdd)); err != nil {
		writeAckErr(w, err)
		return
	}
	if len(toAdd) > 0 {
		if err := s.AckCart.AddAckCart(ctx, toAdd); err != nil {
			writeAckErr(w, err)
			return
		}
	}
	s.writeAckList(ctx, w, skippedInCart, skippedActioned)
}

func (s *IntentPageServer) removeAck(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if err := s.AckCart.RemoveAckCart(ctx, hash); err != nil {
		writeAckErr(w, err)
		return
	}
	s.listAck(ctx, w)
}

func writeAckErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeInvalid {
		status = http.StatusBadRequest
	}
	writeAckJSON(w, status, ackResponse{Error: strings.TrimSpace(err.Error())})
}

func writeAckJSON(w http.ResponseWriter, status int, body ackResponse) {
	if body.Items == nil && status == http.StatusOK {
		body.Items = []ackJSONItem{}
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode ack reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

type ackJobResponse struct {
	OK     bool                 `json:"ok"`
	JobID  string               `json:"job_id,omitempty"`
	Status string               `json:"status,omitempty"`
	Job    *mailack.JobArtifact `json:"job,omitempty"`
	Error  string               `json:"error,omitempty"`
}

func (s *IntentPageServer) serveAckCheckout(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.AckJobs == nil {
		writeAckJobJSON(w, http.StatusServiceUnavailable, ackJobResponse{Error: "ack job needs mail overview serve with pimdir"})
		return
	}
	if r.Method != http.MethodPost {
		writeAckJobJSON(w, http.StatusMethodNotAllowed, ackJobResponse{Error: "use POST"})
		return
	}
	if s.Sync != nil && s.Sync.Running() {
		writeAckJobJSON(w, http.StatusConflict, ackJobResponse{Error: "wait for mail sync to finish"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeAckJobJSON(w, http.StatusInternalServerError, ackJobResponse{Error: "ack request needs a context"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()
	r.Body = http.MaxBytesReader(w, r.Body, cartMaxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeAckJobJSON(w, http.StatusBadRequest, ackJobResponse{Error: "could not read checkout body"})
		return
	}
	var in cartJobApplyRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeAckJobJSON(w, http.StatusBadRequest, ackJobResponse{Error: "checkout body must be JSON"})
		return
	}
	if strings.TrimSpace(in.Confirm) != hoststore.AckJobConfirmToken {
		writeAckJobJSON(w, http.StatusBadRequest, ackJobResponse{Error: "confirm must be seen-at-source"})
		return
	}
	job, err := s.AckJobs.Checkout(ctx, in.Confirm)
	if err != nil {
		body := ackJobResponse{Error: strings.TrimSpace(err.Error())}
		if job.ID != "" {
			body.Job = &job
			body.JobID = job.ID
			body.Status = job.Status
		}
		writeAckJobJSON(w, cartJobHTTPStatus(err), body)
		return
	}
	status := http.StatusOK
	if job.Status == hoststore.DeleteJobStatusApplying {
		status = http.StatusAccepted
	}
	writeAckJobJSON(w, status, ackJobResponse{OK: true, JobID: job.ID, Status: job.Status, Job: &job})
}

func (s *IntentPageServer) serveAckJobs(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.AckJobs == nil {
		writeAckJobJSON(w, http.StatusServiceUnavailable, ackJobResponse{Error: "ack job needs mail overview serve with pimdir"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeAckJobJSON(w, http.StatusInternalServerError, ackJobResponse{Error: "ack request needs a context"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()
	rest := strings.TrimPrefix(r.URL.Path, "/ack/jobs")
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		if r.Method != http.MethodGet {
			writeAckJobJSON(w, http.StatusMethodNotAllowed, ackJobResponse{Error: "use GET"})
			return
		}
		job, err := s.AckJobs.LatestJob(ctx)
		if err != nil {
			if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeNotFound {
				writeAckJobJSON(w, http.StatusNotFound, ackJobResponse{Error: "no jobs"})
				return
			}
			writeAckJobJSON(w, cartJobHTTPStatus(err), ackJobResponse{Error: strings.TrimSpace(err.Error())})
			return
		}
		writeAckJobJSON(w, http.StatusOK, ackJobResponse{OK: true, JobID: job.ID, Status: job.Status, Job: &job})
		return
	}
	if r.Method != http.MethodGet {
		writeAckJobJSON(w, http.StatusMethodNotAllowed, ackJobResponse{Error: "use GET"})
		return
	}
	job, err := s.AckJobs.GetJob(ctx, rest)
	if err != nil {
		writeAckJobJSON(w, cartJobHTTPStatus(err), ackJobResponse{Error: strings.TrimSpace(err.Error())})
		return
	}
	writeAckJobJSON(w, http.StatusOK, ackJobResponse{OK: true, JobID: job.ID, Status: job.Status, Job: &job})
}

func writeAckJobJSON(w http.ResponseWriter, status int, body ackJobResponse) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode ack job reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
