package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type keepJSONItem struct {
	ObjectHash string `json:"object_hash"`
	Sender     string `json:"sender"`
	Subject    string `json:"subject"`
	Date       string `json:"date,omitempty"`
}

type keepAddRequest struct {
	Items []keepJSONItem `json:"items"`
}

type keepResponse struct {
	OK              bool           `json:"ok"`
	Items           []keepJSONItem `json:"items,omitempty"`
	Count           int            `json:"count"`
	SkippedActioned int            `json:"skipped_actioned,omitempty"`
	Error           string         `json:"error,omitempty"`
}

// OutcomeStore persists keep labels.
type OutcomeStore interface {
	maildelete.OutcomeWriter
	ListOutcomes(ctx context.Context) ([]hoststore.MessageOutcome, error)
}

func (s *IntentPageServer) serveKeep(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.Outcomes == nil {
		writeKeepJSON(w, http.StatusServiceUnavailable, keepResponse{Error: "keep labels need mail overview serve with ledger"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeKeepJSON(w, http.StatusInternalServerError, keepResponse{Error: "nil request context"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		s.listKeep(ctx, w)
	case http.MethodPost:
		s.addKeep(ctx, w, r)
	default:
		writeKeepJSON(w, http.StatusMethodNotAllowed, keepResponse{Error: "GET or POST required"})
	}
}

func (s *IntentPageServer) listKeep(ctx context.Context, w http.ResponseWriter) {
	rows, err := s.Outcomes.ListOutcomes(ctx)
	if err != nil {
		writeKeepErr(w, err)
		return
	}
	out := make([]keepJSONItem, 0)
	for _, row := range rows {
		if strings.TrimSpace(row.Outcome) != hoststore.OutcomeKept {
			continue
		}
		out = append(out, keepJSONItem{ObjectHash: row.ObjectHash, Sender: row.Sender})
	}
	writeKeepJSON(w, http.StatusOK, keepResponse{OK: true, Items: out, Count: len(out)})
}

func (s *IntentPageServer) addKeep(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, cartMaxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeKeepJSON(w, http.StatusBadRequest, keepResponse{Error: "could not read keep items"})
		return
	}
	var in keepAddRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeKeepJSON(w, http.StatusBadRequest, keepResponse{Error: "keep items must be JSON"})
		return
	}
	actioned := map[string]maildelete.ActionedInfo{}
	lookup := s.actionedLookup()
	if lookup != nil {
		hashes := make([]string, 0, len(in.Items))
		for _, row := range in.Items {
			hashes = append(hashes, row.ObjectHash)
		}
		got, aerr := lookup.LookupActioned(ctx, hashes)
		if aerr != nil {
			writeKeepErr(w, aerr)
			return
		}
		actioned = got
	}
	skippedActioned := 0
	kept := 0
	for _, row := range in.Items {
		h := strings.TrimSpace(row.ObjectHash)
		if h == "" {
			continue
		}
		if _, ok := actioned[h]; ok {
			skippedActioned++
			continue
		}
		if err := s.Outcomes.UpsertOutcome(ctx, hoststore.MessageOutcome{
			ObjectHash: h,
			Outcome:    hoststore.OutcomeKept,
			Source:     hoststore.OutcomeSourceOverviewKeep,
			Sender:     strings.TrimSpace(row.Sender),
		}); err != nil {
			writeKeepErr(w, err)
			return
		}
		kept++
	}
	s.listKeepAfterPost(ctx, w, skippedActioned, kept)
}

func (s *IntentPageServer) listKeepAfterPost(ctx context.Context, w http.ResponseWriter, skippedActioned, added int) {
	rows, err := s.Outcomes.ListOutcomes(ctx)
	if err != nil {
		writeKeepErr(w, err)
		return
	}
	out := make([]keepJSONItem, 0)
	for _, row := range rows {
		if strings.TrimSpace(row.Outcome) != hoststore.OutcomeKept {
			continue
		}
		out = append(out, keepJSONItem{ObjectHash: row.ObjectHash, Sender: row.Sender})
	}
	_ = added
	writeKeepJSON(w, http.StatusOK, keepResponse{
		OK: true, Items: out, Count: len(out), SkippedActioned: skippedActioned,
	})
}

func writeKeepErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeInvalid {
		status = http.StatusBadRequest
	}
	writeKeepJSON(w, status, keepResponse{Error: strings.TrimSpace(err.Error())})
}

func writeKeepJSON(w http.ResponseWriter, status int, body keepResponse) {
	if body.Items == nil && status == http.StatusOK {
		body.Items = []keepJSONItem{}
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode keep reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
