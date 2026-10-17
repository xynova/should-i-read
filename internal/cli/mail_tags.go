package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/mailtags"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func newMailTagsCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Regenerate the mail tag register from the classify ledger",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailTags(cmd, opts, repoRoot)
		},
	}
	return cmd
}

func runMailTags(cmd *cobra.Command, opts *rootOptions, repoRoot string) error {
	const op = "cli.mail.tags"
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	registerPath, err := config.DefaultTagRegisterPath()
	if err != nil {
		return wrapCLIHop(err, op, "register path")
	}
	depPath, err := config.DefaultTagDeprecationsPath()
	if err != nil {
		return wrapCLIHop(err, op, "deprecations path")
	}
	seedPath := config.SeedTagListPath(repoRoot)
	if err := mailtags.EnsureFiles(registerPath, depPath, seedPath); err != nil {
		return wrapCLIHop(err, op, "ensure tag files")
	}
	cat, err := mailtags.CreateCatalog(mailtags.Config{
		RegisterPath:     registerPath,
		DeprecationsPath: depPath,
		SeedPath:         seedPath,
	})
	if err != nil {
		return wrapCLIHop(err, op, "load tag catalog")
	}
	now := time.Now().UTC()
	store, err := hoststore.CreateStore(hoststore.StoreConfig{Path: cfg.StorePath, Now: now})
	if err != nil {
		return wrapCLIHop(err, op, "open store")
	}
	defer func() { _ = store.Close() }()
	path, err := mailreport.RegenerateTagRegister(ctx, store, cat)
	if err != nil {
		return wrapCLIHop(err, op, "regenerate register")
	}
	if wantJSON(cmd) {
		shapes, err := store.ListCompleteShapes(ctx)
		if err != nil {
			return wrapCLIHop(err, op, "list shapes")
		}
		counts := mailtags.CountTagsFromShapes(shapes)
		for _, id := range cat.SeedIDs() {
			if _, ok := counts[id]; !ok {
				counts[id] = 0
			}
		}
		payload := map[string]interface{}{
			"register_path": path,
			"active":        counts,
		}
		return printJSON(cmd.OutOrStdout(), payload)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Mail tag register: %s\n", path)
	shapes, err := store.ListCompleteShapes(ctx)
	if err != nil {
		return wrapCLIHop(err, op, "list shapes")
	}
	counts := mailtags.CountTagsFromShapes(shapes)
	for _, id := range cat.SeedIDs() {
		if _, ok := counts[id]; !ok {
			counts[id] = 0
		}
	}
	if len(counts) == 0 {
		b.WriteString("No tagged complete rows yet (seed tags listed at count 0 after first classify).\n")
	} else {
		fmt.Fprintf(&b, "%d active tag ids\n", len(counts))
	}
	return writeCLILine(cmd.OutOrStdout(), strings.TrimRight(b.String(), "\n"))
}
