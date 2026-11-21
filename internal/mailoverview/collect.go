package mailoverview

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Collect gathers complete rows from the ledger and mail summaries from pimdir.
func Collect(ctx context.Context, store *hoststore.Store, mail *pimdir.Reader) ([]hoststore.OverviewRow, map[string]pimdir.EmailSummary, error) {
	const op = "mailoverview.Collect"
	if store == nil {
		return nil, nil, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	rows, err := store.ListCompleteOverviewRows(ctx)
	if err != nil {
		return nil, nil, err
	}
	if mail == nil {
		return rows, map[string]pimdir.EmailSummary{}, nil
	}
	hashes := make([]string, 0, len(rows))
	for _, r := range rows {
		h := strings.TrimSpace(r.ObjectHash)
		if h != "" {
			hashes = append(hashes, h)
		}
	}
	m, err := mail.SummariesByObjectHash(ctx, hashes)
	if err != nil {
		return nil, nil, err
	}
	return rows, m, nil
}
