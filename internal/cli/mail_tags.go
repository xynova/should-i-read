package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/behaviorengineering/taxonomy/pkg/tags"
	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const mailTagsPathLabelWidth = 12

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

func loadMailTagsCatalog(repoRoot string) (registerPath, depPath, seedPath string, cat *tags.Catalog, err error) {
	const op = "cli.mail.tags"
	registerPath, err = config.DefaultTagRegisterPath()
	if err != nil {
		return "", "", "", nil, wrapCLIHop(err, op, "register path")
	}
	depPath, err = config.DefaultTagDeprecationsPath()
	if err != nil {
		return "", "", "", nil, wrapCLIHop(err, op, "deprecations path")
	}
	seedPath = config.SeedTagListPath(repoRoot)
	if err := tags.EnsureFiles(registerPath, depPath, seedPath); err != nil {
		return "", "", "", nil, wrapCLIHop(err, op, "ensure tag files")
	}
	cat, err = tags.CreateCatalog(mailreport.MailTagCatalogConfig(registerPath, depPath, seedPath))
	if err != nil {
		return "", "", "", nil, wrapCLIHop(err, op, "load tag catalog")
	}
	return registerPath, depPath, seedPath, cat, nil
}

func mailTagsJudgeFromConfig(ctx context.Context, cfg config.Config) (*polypus.Client, string) {
	client, clientErr := polypus.Create(cfg.PolypusBaseURL, nil)
	if clientErr != nil {
		return nil, ""
	}
	health, healthErr := client.Check(ctx)
	if healthErr != nil {
		return nil, ""
	}
	resolveCtx, cancel := context.WithTimeout(ctx, classifyModelResolveTimeout)
	defer cancel()
	judgeModel, _, err := mailreport.ResolveJudgeModel(resolveCtx, client, health.ModelIDs, cfg.PolypusJudgeModel)
	if err != nil {
		return nil, ""
	}
	return client, judgeModel
}

func mailTagsShapeCounts(ctx context.Context, store *hoststore.Store, cat *tags.Catalog) (map[string]int, error) {
	listCtx, listCancel := context.WithTimeout(ctx, hostStoreOpTimeout)
	defer listCancel()
	shapes, err := store.ListCompleteShapes(listCtx)
	if err != nil {
		return nil, err
	}
	counts := tags.CountTagsFromShapes(shapes)
	for _, id := range cat.SeedIDs() {
		if _, ok := counts[id]; !ok {
			counts[id] = 0
		}
	}
	return counts, nil
}

func assembleMailTagsReport(cfg config.Config, result mailreport.CurateResult, counts map[string]int, registerPath, depPath, seedPath string) mailTagsReport {
	regPath := result.RegisterPath
	if regPath == "" {
		regPath = registerPath
	}
	return mailTagsReport{
		RegisterPath:     regPath,
		DeprecationsPath: depPath,
		SeedPath:         seedPath,
		StorePath:        cfg.StorePath,
		Rewrote:          result.Rewrote,
		Dropped:          result.Dropped,
		Merged:           result.Merged,
		Skipped:          result.Skipped,
		JudgeSkipped:     result.JudgeSkipped,
		ActiveTagIDs:     len(counts),
		Error:            result.Error,
	}
}

type mailTagsReport struct {
	RegisterPath     string            `json:"register_path,omitempty"`
	DeprecationsPath string            `json:"deprecations_path,omitempty"`
	SeedPath         string            `json:"seed_path,omitempty"`
	StorePath        string            `json:"store_path,omitempty"`
	Rewrote          int               `json:"rewrote"`
	Dropped          []string          `json:"dropped,omitempty"`
	Merged           map[string]string `json:"merged,omitempty"`
	Skipped          []string          `json:"skipped,omitempty"`
	JudgeSkipped     bool              `json:"judge_skipped,omitempty"`
	ActiveTagIDs     int               `json:"active_tag_ids"`
	Error            string            `json:"error,omitempty"`
}

func runMailTags(cmd *cobra.Command, opts *rootOptions, repoRoot string) error {
	const op = "cli.mail.tags"
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	parent := cmd.Context()
	if parent == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	tagCtx, tagCancel := context.WithTimeout(parent, mailreport.TagPassTimeout)
	defer tagCancel()
	registerPath, depPath, seedPath, cat, err := loadMailTagsCatalog(repoRoot)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	store, err := hoststore.CreateStore(hoststore.StoreConfig{Path: cfg.StorePath, Now: now})
	if err != nil {
		return wrapCLIHop(err, op, "open store")
	}
	defer func() { _ = store.Close() }()
	judgeClient, judgeModel := mailTagsJudgeFromConfig(parent, cfg)
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
	result, err := curator.Run(tagCtx)
	if err != nil {
		return wrapCLIHop(err, op, "curate tags")
	}
	if loaded := curator.Catalog(); loaded != nil {
		cat = loaded
	}
	counts, err := mailTagsShapeCounts(parent, store, cat)
	if err != nil {
		return wrapCLIHop(err, op, "list shapes")
	}
	report := assembleMailTagsReport(cfg, result, counts, registerPath, depPath, seedPath)
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), report)
	}
	return writeCLILine(cmd.OutOrStdout(), strings.TrimRight(formatMailTags(report), "\n"))
}

func formatMailTags(report mailTagsReport) string {
	var b strings.Builder
	writeMailTagsPath(&b, "Register", report.RegisterPath)
	writeMailTagsPath(&b, "Deprecations", report.DeprecationsPath)
	writeMailTagsPath(&b, "Seed", report.SeedPath)
	writeMailTagsPath(&b, "Ledger", report.StorePath)
	b.WriteByte('\n')
	if report.Rewrote > 0 {
		fmt.Fprintf(&b, "Rewrote %d classify row shape(s)\n", report.Rewrote)
	}
	if report.ActiveTagIDs == 0 {
		b.WriteString("No tagged complete rows yet (seed tags listed at count 0 after first classify).\n")
	} else {
		fmt.Fprintf(&b, "%d active tag ids\n", report.ActiveTagIDs)
	}
	if report.JudgeSkipped {
		b.WriteString(clui.Hint("Judge skipped (Polypus down or no judge model)"))
		b.WriteByte('\n')
	}
	writeMailTagsIDList(&b, "Dropped", report.Dropped)
	writeMailTagsMerged(&b, report.Merged)
	writeMailTagsIDList(&b, "Skipped", report.Skipped)
	return clui.FormatBox("Mail tags", strings.TrimRight(b.String(), "\n"))
}

func writeMailTagsPath(b *strings.Builder, label, path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "(unset)"
	}
	fmt.Fprintf(b, "%s  %s\n", clui.Label(fmt.Sprintf("%-*s", mailTagsPathLabelWidth, label)), clui.Muted(path))
}

func writeMailTagsIDList(b *strings.Builder, heading string, ids []string) {
	if len(ids) == 0 {
		return
	}
	b.WriteByte('\n')
	fmt.Fprintf(b, "%s (%d)\n", heading, len(ids))
	b.WriteString("  ")
	b.WriteString(strings.Join(ids, ", "))
	b.WriteByte('\n')
}

func writeMailTagsMerged(b *strings.Builder, merged map[string]string) {
	if len(merged) == 0 {
		return
	}
	froms := make([]string, 0, len(merged))
	for from := range merged {
		froms = append(froms, from)
	}
	sort.Strings(froms)
	b.WriteByte('\n')
	fmt.Fprintf(b, "Merged (%d)\n", len(froms))
	for _, from := range froms {
		fmt.Fprintf(b, "  %s -> %s\n", from, merged[from])
	}
}
