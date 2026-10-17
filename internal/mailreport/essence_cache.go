package mailreport

import (
	"context"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type essenceCacheAdapter struct {
	store *hoststore.Store
	fp    hoststore.EssenceFingerprint
}

// CreateEssenceCache implements harness.EssenceCache on the host essence_cache table.
func CreateEssenceCache(store *hoststore.Store, systemPrompt, essenceModel, escalateModel string) (harness.EssenceCache, error) {
	const op = "mailreport.CreateEssenceCache"
	if store == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	return &essenceCacheAdapter{
		store: store,
		fp:    essenceFingerprint(essenceModel, escalateModel, systemPrompt),
	}, nil
}

func (a *essenceCacheAdapter) Get(ctx context.Context, key string) (harness.EssenceOut, bool, error) {
	row, hit, err := a.store.GetEssence(ctx, key, a.fp)
	if err != nil || !hit {
		return harness.EssenceOut{}, hit, err
	}
	return essenceOutFromRow(row), true, nil
}

func (a *essenceCacheAdapter) Put(ctx context.Context, key string, out harness.EssenceOut) error {
	return a.store.PutEssence(ctx, key, a.fp, essenceRowFromOut(out))
}
