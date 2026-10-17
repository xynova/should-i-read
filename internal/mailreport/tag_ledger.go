package mailreport

import (
	"context"

	"github.com/behaviorengineering/taxonomy/pkg/tags"

	"github.com/xynova/should-i-read/internal/hoststore"
)

type shapeLedger struct {
	store *hoststore.Store
}

func (l shapeLedger) ListComplete(ctx context.Context) ([]tags.Row, error) {
	rows, err := l.store.ListCompleteShapeRows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]tags.Row, 0, len(rows))
	for _, r := range rows {
		out = append(out, tags.Row{ID: r.ObjectHash, Shape: r.Shape})
	}
	return out, nil
}

func (l shapeLedger) Rewrite(ctx context.Context, id, shape string) error {
	return l.store.RewriteShape(ctx, id, shape)
}
