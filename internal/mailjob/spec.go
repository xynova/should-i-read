package mailjob

import "github.com/xynova/should-i-read/internal/hoststore"

// Kind identifies the leaf mailbox action for an apply job.
type Kind string

const (
	// KindSeen marks messages Seen at the source (Acked cart).
	KindSeen Kind = "seen_at_source"
	// KindTrash moves messages to Trash at the source (deletion cart).
	KindTrash Kind = "trash_at_source"
)

// Spec is the per-kind apply contract (status strings, confirm token, spans).
type Spec struct {
	Kind             Kind
	ConfirmToken     string
	SuccessStatus    string
	FailedStatus     string
	UnresolvedStatus string
	PlannedStatus    string
	RefusedStatus    string
	JobApplying      string
	JobDone          string
	JobPartial       string
	JobPlanned       string
	Outcome          string
	OutcomeSource    string
	Mode             string
	SpanApply        string
	SpanHop          string
	AttrPrefix       string
	CountAttr        string
}

// SeenSpec is the Acked mark-read apply contract.
func SeenSpec() Spec {
	return Spec{
		Kind:             KindSeen,
		ConfirmToken:     hoststore.AckJobConfirmToken,
		SuccessStatus:    hoststore.AckJobItemSeen,
		FailedStatus:     hoststore.AckJobItemFailed,
		UnresolvedStatus: hoststore.AckJobItemUnresolved,
		PlannedStatus:    hoststore.AckJobItemPlanned,
		RefusedStatus:    hoststore.AckJobItemRefused,
		JobApplying:      hoststore.DeleteJobStatusApplying,
		JobDone:          hoststore.DeleteJobStatusDone,
		JobPartial:       hoststore.DeleteJobStatusPartial,
		JobPlanned:       hoststore.DeleteJobStatusPlanned,
		Outcome:          hoststore.OutcomeAcked,
		OutcomeSource:    hoststore.OutcomeSourceAckJob,
		Mode:             string(KindSeen),
		SpanApply:        "mailack.Apply",
		SpanHop:          "mailack.FlagAdd",
		AttrPrefix:       "ack",
		CountAttr:        "seen",
	}
}

// TrashSpec is the Cart trash-at-source apply contract.
func TrashSpec() Spec {
	return Spec{
		Kind:             KindTrash,
		ConfirmToken:     hoststore.DeleteJobConfirmToken,
		SuccessStatus:    hoststore.DeleteJobItemTrashed,
		FailedStatus:     hoststore.DeleteJobItemFailed,
		UnresolvedStatus: hoststore.DeleteJobItemUnresolved,
		PlannedStatus:    hoststore.DeleteJobItemPlanned,
		RefusedStatus:    hoststore.DeleteJobItemRefused,
		JobApplying:      hoststore.DeleteJobStatusApplying,
		JobDone:          hoststore.DeleteJobStatusDone,
		JobPartial:       hoststore.DeleteJobStatusPartial,
		JobPlanned:       hoststore.DeleteJobStatusPlanned,
		Outcome:          hoststore.OutcomeTrashedAtSource,
		OutcomeSource:    hoststore.OutcomeSourceDeleteJob,
		Mode:             string(KindTrash),
		SpanApply:        "maildelete.Apply",
		SpanHop:          "maildelete.MessageDelete",
		AttrPrefix:       "delete",
		CountAttr:        "trashed",
	}
}
