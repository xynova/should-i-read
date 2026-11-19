package mailack

import (
	"context"

	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/mailjob"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type seenHopper struct {
	flagger FlagAdder
}

func (h seenHopper) Hop(ctx context.Context, folder, envelopeID string) error {
	const op = "mailack.seenHopper.Hop"
	if h.flagger == nil {
		return sirerr.New(sirerr.CodeUnavailable, op, himalayaMissingHint)
	}
	res, err := h.flagger.FlagAdd(ctx, folder, envelopeID, pimalaya.FlagSeen)
	if err != nil {
		return err
	}
	count, flags, perr := ParseFlagAddReport(res.Raw)
	if perr != nil {
		return perr
	}
	if !FlagAddSuccess(count, flags) {
		return sirerr.New(sirerr.CodeFailed, op, "unexpected flag add")
	}
	return nil
}

type pimdirResolver struct {
	pim maildelete.PimdirSummaries
}

func (p pimdirResolver) Resolve(ctx context.Context, hashes []string) ([]mailjob.Binding, error) {
	in, err := maildelete.ResolveJobBindings(ctx, p.pim, hashes)
	if err != nil {
		return nil, err
	}
	return mailjob.BindingsFromDelete(in), nil
}
