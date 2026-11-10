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

type cartJobResponse struct {
	OK     bool                    `json:"ok"`
	JobID  string                  `json:"job_id,omitempty"`
	Status string                  `json:"status,omitempty"`
	Job    *maildelete.JobArtifact `json:"job,omitempty"`
	Error  string                  `json:"error,omitempty"`
}

type cartJobApplyRequest struct {
	Confirm string `json:"confirm"`
}

func (s *IntentPageServer) serveCartJobs(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.Jobs == nil {
		writeCartJobJSON(w, http.StatusServiceUnavailable, cartJobResponse{Error: "deletion job needs mail overview serve with pimdir"})
		return
	}
	parent := r.Context()
	if parent == nil {
		writeCartJobJSON(w, http.StatusInternalServerError, cartJobResponse{Error: "nil request context"})
		return
	}
	ctx, cancel := context.WithTimeout(parent, cartHTTPTimeout)
	defer cancel()

	rest := strings.TrimPrefix(r.URL.Path, "/cart/jobs")
	rest = strings.TrimPrefix(rest, "/")
	if rest == "" {
		switch r.Method {
		case http.MethodPost:
			s.createCartJob(ctx, w)
		case http.MethodGet:
			s.latestCartJob(ctx, w)
		default:
			writeCartJobJSON(w, http.StatusMethodNotAllowed, cartJobResponse{Error: "POST or GET required"})
		}
		return
	}
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
		writeCartJobJSON(w, http.StatusBadRequest, cartJobResponse{Error: "missing job id"})
		return
	}
	if len(parts) == 2 && parts[1] == "apply" {
		s.applyCartJob(ctx, w, r, id)
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeCartJobJSON(w, http.StatusMethodNotAllowed, cartJobResponse{Error: "GET required"})
		return
	}
	s.getCartJob(ctx, w, id)
}

func (s *IntentPageServer) createCartJob(ctx context.Context, w http.ResponseWriter) {
	job, err := s.Jobs.StartJob(ctx)
	if err != nil {
		writeCartJobErr(w, err)
		return
	}
	writeCartJobJSON(w, http.StatusOK, cartJobResponse{OK: true, JobID: job.ID, Status: job.Status, Job: &job})
}

func (s *IntentPageServer) latestCartJob(ctx context.Context, w http.ResponseWriter) {
	job, err := s.Jobs.LatestJob(ctx)
	if err != nil {
		if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeNotFound {
			writeCartJobJSON(w, http.StatusNotFound, cartJobResponse{Error: "no jobs"})
			return
		}
		writeCartJobErr(w, err)
		return
	}
	writeCartJobJSON(w, http.StatusOK, cartJobResponse{OK: true, JobID: job.ID, Status: job.Status, Job: &job})
}

func (s *IntentPageServer) getCartJob(ctx context.Context, w http.ResponseWriter, id string) {
	job, err := s.Jobs.GetJob(ctx, id)
	if err != nil {
		writeCartJobErr(w, err)
		return
	}
	writeCartJobJSON(w, http.StatusOK, cartJobResponse{OK: true, JobID: job.ID, Status: job.Status, Job: &job})
}

func (s *IntentPageServer) applyCartJob(ctx context.Context, w http.ResponseWriter, r *http.Request, id string) {
	if r.Method == http.MethodGet {
		writeCartJobJSON(w, http.StatusMethodNotAllowed, cartJobResponse{Error: "POST required"})
		return
	}
	if r.Method != http.MethodPost {
		writeCartJobJSON(w, http.StatusMethodNotAllowed, cartJobResponse{Error: "POST required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, cartMaxBody)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeCartJobJSON(w, http.StatusBadRequest, cartJobResponse{Error: "could not read apply body"})
		return
	}
	var in cartJobApplyRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeCartJobJSON(w, http.StatusBadRequest, cartJobResponse{Error: "apply body must be JSON"})
		return
	}
	if strings.TrimSpace(in.Confirm) != hoststore.DeleteJobConfirmToken {
		writeCartJobJSON(w, http.StatusBadRequest, cartJobResponse{Error: "confirm token mismatch"})
		return
	}
	job, err := s.Jobs.GetJob(ctx, id)
	if err != nil {
		writeCartJobErr(w, err)
		return
	}
	switch job.Status {
	case hoststore.DeleteJobStatusApplying:
		writeCartJobJSON(w, http.StatusAccepted, cartJobResponse{OK: true, JobID: job.ID, Status: job.Status, Job: &job})
		return
	case hoststore.DeleteJobStatusDone:
		writeCartJobJSON(w, http.StatusOK, cartJobResponse{OK: true, JobID: job.ID, Status: job.Status, Job: &job})
		return
	case hoststore.DeleteJobStatusPlanned, hoststore.DeleteJobStatusPartial:
		if err := s.Jobs.ApplyAsync(id, in.Confirm); err != nil {
			writeCartJobErr(w, err)
			return
		}
		job.Status = hoststore.DeleteJobStatusApplying
		writeCartJobJSON(w, http.StatusAccepted, cartJobResponse{OK: true, JobID: job.ID, Status: hoststore.DeleteJobStatusApplying, Job: &job})
		return
	default:
		writeCartJobJSON(w, http.StatusConflict, cartJobResponse{Error: "job cannot apply in status " + job.Status})
	}
}

func writeCartJobErr(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if c, ok := sirerr.AsCode(err); ok {
		switch c {
		case sirerr.CodeInvalid:
			status = http.StatusBadRequest
		case sirerr.CodeNotFound:
			status = http.StatusNotFound
		case sirerr.CodeUnavailable:
			status = http.StatusServiceUnavailable
		}
	}
	writeCartJobJSON(w, status, cartJobResponse{Error: strings.TrimSpace(err.Error())})
}

func writeCartJobJSON(w http.ResponseWriter, status int, body cartJobResponse) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "encode cart job reply", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}
