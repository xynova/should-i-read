package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/behaviorengineering/taxonomy/pkg/tags"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/polypus"
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
	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
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
	if err := tags.EnsureFiles(registerPath, depPath, seedPath); err != nil {
		return wrapCLIHop(err, op, "ensure tag files")
	}
	cat, err := tags.CreateCatalog(mailreport.MailTagCatalogConfig(registerPath, depPath, seedPath))
	if err != nil {
		return wrapCLIHop(err, op, "load tag catalog")
	}
	now := time.Now().UTC()
	store, err := hoststore.CreateStore(hoststore.StoreConfig{Path: cfg.StorePath, Now: now})
	if err != nil {
		return wrapCLIHop(err, op, "open store")
	}
	defer func() { _ = store.Close() }()
	var judgeClient *polypus.Client
	var judgeModel string
	client, clientErr := polypus.Create(cfg.PolypusBaseURL, nil)
	if clientErr == nil {
		health, healthErr := client.Check(parent)
		if healthErr == nil {
			resolveCtx, cancel := context.WithTimeout(parent, classifyModelResolveTimeout)
			judgeModel, _, err = mailreport.ResolveJudgeModel(resolveCtx, client, health.ModelIDs, cfg.PolypusJudgeModel)
			cancel()
			if err == nil {
				judgeClient = client
			}
		}
	}
	curator, err := mailreport.CreateTagCurator(mailreport.TagCuratorConfig{
		Store:      store,
		Catalog:    cat,
		Client:     judgeClient,
		JudgeModel: judgeModel,
		MinScore:   cfg.Taxonomy.MinJudgeScore,
	})
	if err != nil {
		return wrapCLIHop(err, op, "create tag curator")
	}
	result, err := curator.Run(parent)
	if err != nil {
		return wrapCLIHop(err, op, "curate tags")
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), result)
	}
	var b strings.Builder
	path := result.RegisterPath
	if path == "" {
		path = registerPath
	}
	fmt.Fprintf(&b, "Mail tag register: %s\n", path)
	if result.Rewrote > 0 {
		fmt.Fprintf(&b, "rewrote %d classify row shape(s)\n", result.Rewrote)
	}
	if len(result.Dropped) > 0 {
		fmt.Fprintf(&b, "dropped tags: %s\n", strings.Join(result.Dropped, ", "))
	}
	if len(result.Merged) > 0 {
		b.WriteString("merged tags:\n")
		for from, to := range result.Merged {
			fmt.Fprintf(&b, "  %s -> %s\n", from, to)
		}
	}
	if result.JudgeSkipped {
		b.WriteString("jev skipped (polypus down or no judge model)\n")
	}
	cat = curator.Catalog()
	if cat == nil {
		cat, _ = tags.CreateCatalog(mailreport.MailTagCatalogConfig(registerPath, depPath, seedPath))
	}
	listCtx, listCancel := context.WithTimeout(parent, hostStoreOpTimeout)
	defer listCancel()
	shapes, err := store.ListCompleteShapes(listCtx)
	if err != nil {
		return wrapCLIHop(err, op, "list shapes")
	}
	counts := tags.CountTagsFromShapes(shapes)
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
