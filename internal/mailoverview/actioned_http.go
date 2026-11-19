package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

type actionedResponse struct {
	OK    bool          `json:"ok"`
	Rows  []ActionedRow `json:"rows,omitempty"`
	Error string        `json:"error,omitempty"`
}

func (s *IntentPageServer) serveActioned(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeActionedJSON(w, http.StatusMethodNotAllowed, actionedResponse{Error: "GET required"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeActionedJSON(w, http.StatusInternalServerError, actionedResponse{Error: "nil request context"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()
	lookup := s.actionedLookup()
	if lookup == nil {
		writeActionedJSON(w, http.StatusOK, actionedResponse{OK: true, Rows: []ActionedRow{}})
		return
	}
	s.pageMu.RLock()
	hashes := append([]string(nil), s.Hashes...)
	s.pageMu.RUnlock()
	got, err := lookup.LookupActioned(ctx, hashes)
	if err != nil {
		status := http.StatusInternalServerError
		if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeInvalid {
			status = http.StatusBadRequest
		}
		writeActionedJSON(w, status, actionedResponse{Error: strings.TrimSpace(err.Error())})
		return
	}
	writeActionedJSON(w, http.StatusOK, actionedResponse{OK: true, Rows: GroupActioned(got)})
}

func writeActionedJSON(w http.ResponseWriter, status int, body actionedResponse) {
	if body.Rows == nil && status == http.StatusOK {
		body.Rows = []ActionedRow{}
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode actioned reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
