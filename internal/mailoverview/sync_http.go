package mailoverview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

type syncResponse struct {
	OK      bool   `json:"ok"`
	Status  string `json:"status,omitempty"`
	RunID   string `json:"run_id,omitempty"`
	Account string `json:"account,omitempty"`
	Error   string `json:"error,omitempty"`
	Fetch   int    `json:"fetch,omitempty"`
	Delete  int    `json:"delete,omitempty"`
	Line    string `json:"line,omitempty"`
}

func (s *IntentPageServer) serveSync(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.Sync == nil {
		writeSyncJSON(w, http.StatusServiceUnavailable, syncResponse{Error: "mail sync needs neverest"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		snap := s.Sync.Snapshot()
		writeSyncJSON(w, http.StatusOK, snapshotToResponse(snap, true))
		return
	case http.MethodPost:
		s.postSync(w, r)
		return
	default:
		writeSyncJSON(w, http.StatusMethodNotAllowed, syncResponse{Error: "GET or POST required"})
	}
}

func (s *IntentPageServer) postSync(w http.ResponseWriter, r *http.Request) {
	parent := r.Context()
	if parent == nil {
		writeSyncJSON(w, http.StatusInternalServerError, syncResponse{Error: "nil request context"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, intentMaxBody)
	_, _ = io.Copy(io.Discard, r.Body)
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()
	snap, err := s.Sync.Start(ctx)
	if err != nil {
		if errors.Is(err, errSyncBusy) {
			writeSyncJSON(w, http.StatusConflict, syncResponse{OK: false, Error: errSyncBusy.Error()})
			return
		}
		if errors.Is(err, errSyncOverlap) {
			writeSyncJSON(w, http.StatusConflict, syncResponse{OK: false, Error: errSyncOverlap.Error()})
			return
		}
		writeSyncJSON(w, syncHTTPStatus(err), syncResponse{OK: false, Error: syncClientError(err)})
		return
	}
	writeSyncJSON(w, http.StatusAccepted, snapshotToResponse(snap, true))
}

func snapshotToResponse(snap SyncSnapshot, ok bool) syncResponse {
	return syncResponse{
		OK:      ok,
		Status:  snap.Status,
		RunID:   snap.RunID,
		Account: snap.Account,
		Error:   snap.Error,
		Fetch:   snap.Fetch,
		Delete:  snap.Delete,
		Line:    snap.Line,
	}
}

func syncHTTPStatus(err error) int {
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
	return status
}

func syncClientError(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func writeSyncJSON(w http.ResponseWriter, status int, body syncResponse) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode sync reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
