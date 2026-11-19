package mailjob

import (
	"context"
	"time"

	"github.com/xynova/should-i-read/internal/hoststore"
)

// Item is one applyable job row, independent of ack vs trash tables.
type Item struct {
	ObjectHash string
	Sender     string
	Collection string
	Seq        int64
	LinkID     string
	EnvelopeID string
	Folder     string
	Status     string
	Error      string
}

// Binding is a pimdir resolve result to persist onto a job item.
type Binding struct {
	ObjectHash string
	Collection string
	Seq        int64
	LinkID     string
	EnvelopeID string
	Folder     string
	Status     string
	Error      string
}

// Job is a job header plus items.
type Job struct {
	ID     string
	Status string
	Error  string
	Items  []Item
}

// Store is the ledger surface the apply runner needs.
type Store interface {
	SetJobStatus(ctx context.Context, jobID, status, errText string, confirmedAt *time.Time) error
	ListApplyableItems(ctx context.Context, jobID string) ([]Item, error)
	BindItems(ctx context.Context, jobID string, bindings []Binding) error
	SetItemStatus(ctx context.Context, jobID, objectHash, status, errText string) error
	GetJob(ctx context.Context, jobID string) (Job, error)
	UpsertOutcome(ctx context.Context, o hoststore.MessageOutcome) error
	RemoveCart(ctx context.Context, objectHash string) error
}

// Resolver maps object hashes to folder and envelope ids.
type Resolver interface {
	Resolve(ctx context.Context, hashes []string) ([]Binding, error)
}

// Hopper is the Himalaya leaf (flag Seen or message delete).
type Hopper interface {
	Hop(ctx context.Context, folder, envelopeID string) error
}

// BindingsFromDelete copies delete-job bindings into the runner shape.
func BindingsFromDelete(in []hoststore.DeleteJobBinding) []Binding {
	out := make([]Binding, 0, len(in))
	for _, b := range in {
		out = append(out, Binding{
			ObjectHash: b.ObjectHash,
			Collection: b.Collection,
			Seq:        b.Seq,
			LinkID:     b.LinkID,
			EnvelopeID: b.EnvelopeID,
			Folder:     b.Folder,
			Status:     b.Status,
			Error:      b.Error,
		})
	}
	return out
}

func itemFromAck(it hoststore.AckJobItem) Item {
	return Item{
		ObjectHash: it.ObjectHash,
		Sender:     it.Sender,
		Collection: it.Collection,
		Seq:        it.Seq,
		LinkID:     it.LinkID,
		EnvelopeID: it.EnvelopeID,
		Folder:     it.Folder,
		Status:     it.Status,
		Error:      it.Error,
	}
}

func itemFromDelete(it hoststore.DeleteJobItem) Item {
	return Item{
		ObjectHash: it.ObjectHash,
		Sender:     it.Sender,
		Collection: it.Collection,
		Seq:        it.Seq,
		LinkID:     it.LinkID,
		EnvelopeID: it.EnvelopeID,
		Folder:     it.Folder,
		Status:     it.Status,
		Error:      it.Error,
	}
}

func bindingToAck(b Binding) hoststore.AckJobBinding {
	return hoststore.AckJobBinding{
		ObjectHash: b.ObjectHash,
		Collection: b.Collection,
		Seq:        b.Seq,
		LinkID:     b.LinkID,
		EnvelopeID: b.EnvelopeID,
		Folder:     b.Folder,
		Status:     b.Status,
		Error:      b.Error,
	}
}

func bindingToDelete(b Binding) hoststore.DeleteJobBinding {
	return hoststore.DeleteJobBinding{
		ObjectHash: b.ObjectHash,
		Collection: b.Collection,
		Seq:        b.Seq,
		LinkID:     b.LinkID,
		EnvelopeID: b.EnvelopeID,
		Folder:     b.Folder,
		Status:     b.Status,
		Error:      b.Error,
	}
}
