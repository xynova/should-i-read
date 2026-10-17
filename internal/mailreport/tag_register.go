package mailreport

import (
	"context"

	"github.com/behaviorengineering/taxonomy/pkg/tags"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// RegenerateTagRegister scans complete classify rows and writes the operator register file.
func RegenerateTagRegister(ctx context.Context, store *hoststore.Store, cat *tags.Catalog) (string, error) {
	const op = "mailreport.RegenerateTagRegister"
	if store == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if cat == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil tag catalog")
	}
	path, err := tags.RegenerateRegister(ctx, shapeLedger{store: store}, cat)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "regenerate register")
	}
	return path, nil
}
