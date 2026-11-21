package maildelete

import (
	"context"

	"github.com/xynova/should-i-read/internal/mailjob"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type trashHopper struct {
	deleter TrashDeleter
}

func (h trashHopper) Hop(ctx context.Context, folder, envelopeID string) error {
	const op = "maildelete.trashHopper.Hop"
	if h.deleter == nil {
		return sirerr.New(sirerr.CodeUnavailable, op, pimalaya.HimalayaMissingHint)
	}
	res, err := h.deleter.MessageDelete(ctx, folder, envelopeID)
	if err != nil {
		return err
	}
	action, count, perr := ParseDeleteReport(res.Raw)
	if perr != nil {
		return perr
	}
	if !TrashSuccess(action, count) {
		return sirerr.New(sirerr.CodeFailed, op, "unexpected delete action")
	}
	return nil
}

type pimdirResolver struct {
	pim PimdirSummaries
}

func (p pimdirResolver) Resolve(ctx context.Context, hashes []string) ([]mailjob.Binding, error) {
	in, err := ResolveJobBindings(ctx, p.pim, hashes)
	if err != nil {
		return nil, err
	}
	return mailjob.BindingsFromDelete(in), nil
}
