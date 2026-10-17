package mailreport

import (
	"context"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailtags"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// RegenerateTagRegister scans complete classify rows and writes the operator register file.
func RegenerateTagRegister(ctx context.Context, store *hoststore.Store, cat *mailtags.Catalog) (string, error) {
	const op = "mailreport.RegenerateTagRegister"
	if store == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil store")
	}
	if cat == nil {
		return "", sirerr.New(sirerr.CodeInvalid, op, "nil tag catalog")
	}
	path := cat.RegisterPath()
	if path == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "empty register path")
	}
	shapes, err := store.ListCompleteShapes(ctx)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "list shapes")
	}
	if err := mailtags.WriteRegister(path, shapes, cat.SeedIDs(), cat.Deprecations()); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "write register")
	}
	return path, nil
}
