package maildelete

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	// OutcomeAlreadyInTrash is replica Trash (or a refused Trash bind).
	OutcomeAlreadyInTrash = "already in Trash"
	// OutcomeTrashedAtSource is a job item Himalaya moved to Trash.
	OutcomeTrashedAtSource = "trashed at source"
	// OutcomeAckedAtSource is a job item Himalaya marked seen.
	OutcomeAckedAtSource = "marked read at source"
)

// ActionedInfo is one hash that must leave the live months and stay out of the cart.
type ActionedInfo struct {
	ObjectHash string
	Outcome    string
	Sender     string
	Subject    string
	Date       string
	Folder     string
}

// ClassifyActioned returns an actioned outcome, or empty when the hash stays live.
func ClassifyActioned(jobStatus, collection string) string {
	if IsTrashFolder(collection) {
		return OutcomeAlreadyInTrash
	}
	switch strings.TrimSpace(jobStatus) {
	case hoststore.DeleteJobItemTrashed:
		return OutcomeTrashedAtSource
	case hoststore.DeleteJobItemRefused:
		return OutcomeAlreadyInTrash
	default:
		return ""
	}
}

// OutcomeWriter persists durable message outcomes.
type OutcomeWriter interface {
	UpsertOutcome(ctx context.Context, o hoststore.MessageOutcome) error
}

// OutcomeCodeFromHuman maps actioned UI labels to store codes.
func OutcomeCodeFromHuman(human string) string {
	switch strings.TrimSpace(human) {
	case OutcomeTrashedAtSource:
		return hoststore.OutcomeTrashedAtSource
	case OutcomeAlreadyInTrash:
		return hoststore.OutcomeAlreadyInTrash
	case OutcomeAckedAtSource:
		return hoststore.OutcomeAcked
	default:
		return ""
	}
}

// HumanOutcomeFromCode maps store codes to actioned UI labels.
func HumanOutcomeFromCode(code string) string {
	switch strings.TrimSpace(code) {
	case hoststore.OutcomeTrashedAtSource:
		return OutcomeTrashedAtSource
	case hoststore.OutcomeAlreadyInTrash:
		return OutcomeAlreadyInTrash
	case hoststore.OutcomeAcked:
		return OutcomeAckedAtSource
	default:
		return ""
	}
}

// MergeTrashOutcomes hides hashes that already have a trash ledger row.
func MergeTrashOutcomes(infos map[string]ActionedInfo, outcomes map[string]hoststore.MessageOutcome) map[string]ActionedInfo {
	if infos == nil {
		infos = map[string]ActionedInfo{}
	}
	for hash, row := range outcomes {
		h := strings.TrimSpace(hash)
		if h == "" {
			continue
		}
		if _, ok := infos[h]; ok {
			continue
		}
		human := HumanOutcomeFromCode(row.Outcome)
		if human == "" {
			continue
		}
		infos[h] = ActionedInfo{
			ObjectHash: h,
			Outcome:    human,
			Sender:     strings.TrimSpace(row.Sender),
		}
	}
	return infos
}

func outcomeSource(job hoststore.DeleteJobItem, info ActionedInfo) (source, jobID string) {
	st := strings.TrimSpace(job.Status)
	if st == hoststore.DeleteJobItemTrashed || st == hoststore.DeleteJobItemRefused {
		return hoststore.OutcomeSourceDeleteJob, strings.TrimSpace(job.JobID)
	}
	if IsTrashFolder(info.Folder) {
		return hoststore.OutcomeSourceReplicaFolder, ""
	}
	if info.Outcome == OutcomeAlreadyInTrash && strings.TrimSpace(info.Folder) == "" {
		return hoststore.OutcomeSourceReplicaFolder, ""
	}
	return hoststore.OutcomeSourceDeleteJob, strings.TrimSpace(job.JobID)
}

// PersistActioned writes actioned lookup results into message_outcomes.
func PersistActioned(ctx context.Context, w OutcomeWriter, jobs map[string]hoststore.DeleteJobItem, infos map[string]ActionedInfo) error {
	const op = "maildelete.PersistActioned"
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if w == nil {
		return nil
	}
	if jobs == nil {
		jobs = map[string]hoststore.DeleteJobItem{}
	}
	for hash, info := range infos {
		code := OutcomeCodeFromHuman(info.Outcome)
		if code == "" {
			continue
		}
		job := jobs[hash]
		src, jobID := outcomeSource(job, info)
		if err := w.UpsertOutcome(ctx, hoststore.MessageOutcome{
			ObjectHash: hash,
			Outcome:    code,
			Source:     src,
			JobID:      jobID,
			Sender:     strings.TrimSpace(info.Sender),
		}); err != nil {
			return sirerr.Wrap(err, sirerr.CodeFailed, op, "upsert outcome").With("object_hash", hash)
		}
	}
	return nil
}

// LookupActioned classifies hashes from replica folders and the latest job row.
func LookupActioned(ctx context.Context, pim PimdirSummaries, jobs map[string]hoststore.DeleteJobItem, hashes []string) (map[string]ActionedInfo, error) {
	const op = "maildelete.LookupActioned"
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "missing context deadline")
	}
	if jobs == nil {
		jobs = map[string]hoststore.DeleteJobItem{}
	}
	summaries := map[string]pimdir.EmailSummary{}
	if pim != nil {
		got, err := pim.SummariesByObjectHash(ctx, hashes)
		if err != nil {
			return nil, err
		}
		summaries = got
	}
	out := map[string]ActionedInfo{}
	for _, raw := range hashes {
		h := strings.TrimSpace(raw)
		if h == "" {
			continue
		}
		job := jobs[h]
		sum, found := summaries[h]
		if pim != nil && !found {
			outcome := ClassifyActioned(job.Status, "")
			if outcome == "" {
				outcome = OutcomeAlreadyInTrash
			}
			sender := strings.TrimSpace(job.Sender)
			if sender == "" {
				sender = "(unknown sender)"
			}
			out[h] = ActionedInfo{
				ObjectHash: h,
				Outcome:    outcome,
				Sender:     sender,
				Subject:    strings.TrimSpace(job.Subject),
			}
			continue
		}
		col := strings.TrimSpace(sum.Collection)
		if col == "" {
			col = job.Folder
		}
		if col == "" {
			col = job.Collection
		}
		outcome := ClassifyActioned(job.Status, col)
		if outcome == "" {
			continue
		}
		sender := strings.TrimSpace(sum.Sender)
		if sender == "" {
			sender = strings.TrimSpace(job.Sender)
		}
		subject := strings.TrimSpace(sum.Subject)
		if subject == "" {
			subject = strings.TrimSpace(job.Subject)
		}
		out[h] = ActionedInfo{
			ObjectHash: h,
			Outcome:    outcome,
			Sender:     sender,
			Subject:    subject,
			Date:       strings.TrimSpace(sum.Date),
			Folder:     col,
		}
	}
	return out, nil
}

// LookupActioned classifies hashes using the job store and pimdir replica.
func (s *Service) LookupActioned(ctx context.Context, hashes []string) (map[string]ActionedInfo, error) {
	const op = "maildelete.Service.LookupActioned"
	if s == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil service")
	}
	items, err := s.store.LatestDeleteJobItems(ctx, hashes)
	if err != nil {
		return nil, err
	}
	infos, err := LookupActioned(ctx, s.pimdir, items, hashes)
	if err != nil {
		return nil, err
	}
	if err := PersistActioned(ctx, s.store, items, infos); err != nil {
		return infos, err
	}
	outcomes, err := s.store.GetOutcomes(ctx, hashes)
	if err != nil {
		return nil, err
	}
	return MergeTrashOutcomes(infos, outcomes), nil
}
