package maildelete

import (
	"context"
	"strconv"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// CartRemover drops cart rows after successful trash.
type CartRemover interface {
	RemoveDeleteCart(ctx context.Context, objectHash string) error
}

// PimdirSummaries resolves object hashes to live replica rows.
type PimdirSummaries interface {
	SummariesByObjectHash(ctx context.Context, hashes []string) (map[string]pimdir.EmailSummary, error)
}

// BindingFromSummary maps one pimdir row to a job binding.
func BindingFromSummary(hash string, sum pimdir.EmailSummary, found bool) hoststore.DeleteJobBinding {
	b := hoststore.DeleteJobBinding{ObjectHash: hash}
	if !found {
		b.Status = hoststore.DeleteJobItemUnresolved
		return b
	}
	b.Collection = sum.Collection
	b.Seq = sum.Seq
	b.LinkID = sum.LinkID
	folder := NormalizeFolder(sum.Collection)
	b.Folder = folder
	if folder == "" {
		b.Status = hoststore.DeleteJobItemUnresolved
		return b
	}
	if IsTrashFolder(sum.Collection) {
		b.Status = hoststore.DeleteJobItemRefused
		return b
	}
	if sum.Seq > 0 {
		b.EnvelopeID = strconv.FormatInt(sum.Seq, 10)
	} else if strings.TrimSpace(sum.LinkID) != "" {
		b.EnvelopeID = strings.TrimSpace(sum.LinkID)
	} else {
		b.Status = hoststore.DeleteJobItemUnresolved
		return b
	}
	b.Status = hoststore.DeleteJobItemPlanned
	return b
}

// ResolveJobBindings resolves every hash in a job.
func ResolveJobBindings(ctx context.Context, pim PimdirSummaries, hashes []string) ([]hoststore.DeleteJobBinding, error) {
	const op = "maildelete.ResolveJobBindings"
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if pim == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil pimdir")
	}
	sums, err := pim.SummariesByObjectHash(ctx, hashes)
	if err != nil {
		return nil, err
	}
	out := make([]hoststore.DeleteJobBinding, 0, len(hashes))
	for _, h := range hashes {
		sum, found := sums[h]
		out = append(out, BindingFromSummary(h, sum, found))
	}
	return out, nil
}
