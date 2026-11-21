package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hostevent"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	classifyModelResolveTimeout = 45 * time.Second
	classifyJobTimeout          = 15 * time.Minute
	classifyWindowJobTimeout    = 60 * time.Minute
)

type classifyOpts struct {
	catalogPath     string
	noApply         bool
	maxBody         int
	skipAuthorProbe bool
	skipEmbedProbe  bool
	Events          *hostevent.Bus
}

const postSyncReportLimit = 500

func newMailReportCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		inPath    string
		outPath   string
		limit     int
		store     string
		catalog   string
		noApply   bool
		maxBody   int
		fromDate  string
		toDate    string
		sliceDays int
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Classify recent mail (report-only, Polypus + taxonomy)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailReport(cmd, opts, repoRoot, inPath, outPath, limit, cmd.Flags().Changed("limit"), fromDate, toDate, sliceDays, store, classifyOpts{
				catalogPath: catalog,
				noApply:     noApply,
				maxBody:     maxBody,
			})
		},
	}
	cmd.Flags().StringVar(&inPath, "in", "", "Input mail export JSON (default: list from pimdir)")
	cmd.Flags().StringVar(&outPath, "out", "", "Report path (default tmp/unwanted-report-<ts>.json)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max messages when listing from pimdir (default 25; with --from/--to, no cap unless you set --limit)")
	cmd.Flags().StringVar(&fromDate, "from", "", "Inclusive window start date (YYYY-MM-DD UTC); requires --to")
	cmd.Flags().StringVar(&toDate, "to", "", "Exclusive window end date (YYYY-MM-DD UTC); requires --from")
	cmd.Flags().IntVar(&sliceDays, "slice-days", 7, "Classify window slice length in days when --from/--to are set")
	cmd.Flags().StringVar(&store, "store", "", "Local mail store directory (overrides config)")
	cmd.Flags().StringVar(&catalog, "catalog", "", "Taxonomy catalog YAML path")
	cmd.Flags().BoolVar(&noApply, "no-apply", false, "Do not write catalog mutations to disk")
	cmd.Flags().IntVar(&maxBody, "max-body", 8192, "Max body bytes per message preview")
	return cmd
}

func runMailReport(cmd *cobra.Command, opts *rootOptions, repoRoot string, inPath, outPath string, limit int, limitChanged bool, fromDate, toDate string, sliceDays int, storeDir string, cOpts classifyOpts) error {
	const op = "cli.mail.report"
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	window, err := validateMailReportWindow(inPath, fromDate, toDate)
	if err != nil {
		return wrapCLIHop(err, op, "validate window")
	}
	if sliceDays < 1 {
		return sirerr.New(sirerr.CodeInvalid, op, "slice-days must be at least 1")
	}
	dir := storeDir
	if dir == "" {
		dir = cfg.Pimalaya.PimdirPath
	}
	reader, err := pimdir.OpenStore(dir)
	if err != nil {
		return wrapCLIHop(err, op, "open store")
	}
	runner, err := prepareClassifyRunner(cmd.Context(), cfg, repoRoot, cOpts)
	if err != nil {
		return err
	}
	defer func() { _ = runner.Close() }()
	job := mailReportJob{
		ctx:          cmd.Context(),
		runner:       runner,
		reader:       reader,
		inPath:       inPath,
		window:       window,
		limit:        limit,
		limitChanged: limitChanged,
		maxBody:      cOpts.maxBody,
		collections:  cfg.Taxonomy.Collections,
		sliceDays:    sliceDays,
	}
	var art mailreport.Artifact
	var clusterErr error
	if !window.Zero() {
		art, clusterErr, err = runMailReportWindowed(job)
	} else {
		art, clusterErr, err = runMailReportUnwindowed(job)
	}
	if err != nil {
		return err
	}
	return emitClassifyArtifact(cmd, repoRoot, outPath, art, clusterErr)
}

type mailReportJob struct {
	ctx          context.Context
	runner       *mailreport.Runner
	reader       *pimdir.Reader
	inPath       string
	window       pimdir.DateWindow
	limit        int
	limitChanged bool
	maxBody      int
	collections  []string
	sliceDays    int
}

func runMailReportWindowed(job mailReportJob) (mailreport.Artifact, error, error) {
	const op = "cli.mail.report"
	var art mailreport.Artifact
	slices, err := pimdir.SplitDateWindow(job.window, job.sliceDays)
	if err != nil {
		return art, nil, wrapCLIHop(err, op, "split window")
	}
	art.WindowFrom = job.window.DateFrom()
	art.WindowTo = job.window.DateTo()
	for _, sw := range slices {
		sliceArt, err := classifyReportWindowSlice(job.ctx, job.runner, job.reader, job.inPath, sw, job.limit, job.limitChanged, job.maxBody, job.collections)
		if err != nil {
			return art, nil, wrapCLIHop(err, op, "classify slice")
		}
		mailreport.MergeClassifyArtifacts(&art, sliceArt)
	}
	tagCtx, tagCancel := context.WithTimeout(job.ctx, mailreport.TagPassTimeout)
	defer tagCancel()
	job.runner.RunTagPass(tagCtx, &art)
	var sliceClusters [][]mailreport.ClusterRow
	var clusterErr error
	for _, sw := range slices {
		sliceArt, err := scoreWindowSliceAttention(job, &art, sw)
		if err != nil {
			return art, nil, err
		}
		clusterCtx, clusterCancel := context.WithTimeout(job.ctx, classifyJobTimeout)
		sliceArt, err = job.runner.Cluster(clusterCtx, sliceArt)
		clusterCancel()
		if err != nil {
			clusterErr = err
		}
		sliceClusters = append(sliceClusters, sliceArt.Clusters)
	}
	mergeCtx, mergeCancel := context.WithTimeout(job.ctx, classifyJobTimeout)
	defer mergeCancel()
	rows, clusters, err := job.runner.MergeClusters(mergeCtx, art.Messages, sliceClusters)
	if err != nil {
		clusterErr = err
	} else {
		art.Messages = rows
		art.Clusters = clusters
	}
	priorCtx, priorCancel := context.WithTimeout(job.ctx, classifyJobTimeout)
	art, err = job.runner.ScoreWorthReadPrior(priorCtx, art)
	priorCancel()
	return art, clusterErr, err
}

func scoreWindowSliceAttention(job mailReportJob, art *mailreport.Artifact, sw pimdir.DateWindow) (mailreport.Artifact, error) {
	const op = "cli.mail.report"
	reloadCtx, reloadCancel := context.WithTimeout(job.ctx, hostStoreOpTimeout)
	items, err := loadReportItems(reloadCtx, job.reader, job.inPath, sw, job.limit, job.limitChanged, job.maxBody, job.collections)
	reloadCancel()
	if err != nil {
		return mailreport.Artifact{}, wrapCLIHop(err, op, "reload slice items")
	}
	hashSet := map[string]struct{}{}
	for _, it := range items {
		if h := strings.TrimSpace(it.ObjectHash); h != "" {
			hashSet[h] = struct{}{}
		}
	}
	sliceArt := mailreport.FilterArtifactByHashes(*art, hashSet)
	attCtx, attCancel := context.WithTimeout(job.ctx, classifyJobTimeout)
	sliceArt, err = job.runner.ScoreAttention(attCtx, sliceArt)
	attCancel()
	if err != nil {
		return mailreport.Artifact{}, err
	}
	mailreport.MergeMessageRows(art, sliceArt)
	return sliceArt, nil
}

func runMailReportUnwindowed(job mailReportJob) (mailreport.Artifact, error, error) {
	const op = "cli.mail.report"
	ctx, cancel, err := classifyJobContext(job.ctx, false)
	if err != nil {
		return mailreport.Artifact{}, nil, err
	}
	defer cancel()
	items, err := loadReportItems(ctx, job.reader, job.inPath, job.window, job.limit, job.limitChanged, job.maxBody, job.collections)
	if err != nil {
		return mailreport.Artifact{}, nil, wrapCLIHop(err, op, "load items")
	}
	items = mailreport.DedupeItemsByHash(items)
	art, err := job.runner.Classify(ctx, mailreport.SourceReport, items)
	if err != nil {
		return art, nil, err
	}
	tagCtx, tagCancel := context.WithTimeout(job.ctx, mailreport.TagPassTimeout)
	defer tagCancel()
	job.runner.RunTagPass(tagCtx, &art)
	attCtx, attCancel := context.WithTimeout(job.ctx, classifyJobTimeout)
	defer attCancel()
	art, err = job.runner.ScoreAttention(attCtx, art)
	if err != nil {
		return art, nil, err
	}
	art, clusterErr := job.runner.Cluster(attCtx, art)
	priorCtx, priorCancel := context.WithTimeout(job.ctx, classifyJobTimeout)
	art, err = job.runner.ScoreWorthReadPrior(priorCtx, art)
	priorCancel()
	return art, clusterErr, err
}

func emitClassifyArtifact(cmd *cobra.Command, repoRoot, outPath string, art mailreport.Artifact, clusterErr error) error {
	const op = "cli.mail.report"
	path, err := writeReportArtifact(repoRoot, outPath, art)
	if err != nil {
		return wrapCLIHop(err, op, "write report")
	}
	art.ReportPath = path
	if wantJSON(cmd) {
		if err := printJSON(cmd.OutOrStdout(), art); err != nil {
			return err
		}
	} else if err := writeCLILine(cmd.OutOrStdout(), formatClassifyArtifact(art)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write classify summary")
	}
	if clusterErr != nil {
		return sirerr.Wrap(clusterErr, sirerr.CodeFailed, op, "cluster")
	}
	return nil
}

func runMailClassifyAfterSync(cmd *cobra.Command, opts *rootOptions, repoRoot string, res *pimalaya.Result, noClassify bool, maxBody int, excludeCollections []string) error {
	const op = "cli.mail.classifyAfterSync"
	if noClassify {
		return nil
	}
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	art, down, err := runPostSyncClassify(cmd.Context(), cfg, repoRoot, res, maxBody, excludeCollections, nil)
	if down {
		box := clui.FormatBox("Classify", clui.Muted("Classification skipped (Polypus unavailable)"))
		if wErr := writeCLILine(cmd.OutOrStdout(), box); wErr != nil {
			return sirerr.Wrap(wErr, sirerr.CodeFailed, op, "write classify skip")
		}
		return nil
	}
	if err != nil {
		if isClassifyUnavailable(err) {
			box := clui.FormatBox("Classify", clui.Muted("Classification skipped (Polypus unavailable)"))
			if wErr := writeCLILine(cmd.OutOrStdout(), box); wErr != nil {
				return sirerr.Wrap(wErr, sirerr.CodeFailed, op, "write classify skip")
			}
			return nil
		}
		box := clui.FormatBox("Classify", clui.Muted("Classification after sync failed: "+shortClassifyErr(err)))
		if wErr := writeCLILine(cmd.OutOrStdout(), box); wErr != nil {
			return sirerr.Wrap(wErr, sirerr.CodeFailed, op, "write classify failure")
		}
		return nil
	}
	if art.GeneratedAt == "" {
		return nil
	}
	if wantJSON(cmd) {
		return nil
	}
	summary := formatClassifyArtifact(art)
	if err := writeCLILine(cmd.OutOrStdout(), summary); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write classify summary")
	}
	return nil
}

func continueClassifyAfterSync(rep pimalaya.EngineReport) bool {
	return rep.Recognized && !rep.DryRun
}

func postSyncClassifyItems(ctx context.Context, reader *pimdir.Reader, res *pimalaya.Result, rep pimalaya.EngineReport, maxBody int, excludeCollections, collections []string) ([]mailreport.Item, int, string, error) {
	if rep.Fetch > 0 {
		items, unresolved := itemsFromFetchHunks(ctx, reader, pimalaya.FetchedHunks(res.Raw), maxBody, excludeCollections)
		return items, unresolved, mailreport.SourceSyncFetch, nil
	}
	items, err := loadReportItems(ctx, reader, "", pimdir.DateWindow{}, postSyncReportLimit, true, maxBody, collections)
	if err != nil {
		return nil, 0, "", err
	}
	return dropExcludedItems(items, excludeCollections), 0, mailreport.SourceReport, nil
}

func dropExcludedItems(items []mailreport.Item, exclude []string) []mailreport.Item {
	if len(exclude) == 0 {
		return items
	}
	skip := map[string]struct{}{}
	for _, raw := range exclude {
		name := strings.TrimSpace(raw)
		if name != "" {
			skip[name] = struct{}{}
		}
	}
	out := make([]mailreport.Item, 0, len(items))
	for _, it := range items {
		if _, ok := skip[strings.TrimSpace(it.Collection)]; ok {
			continue
		}
		out = append(out, it)
	}
	return out
}

func runPostSyncClassify(parent context.Context, cfg config.Config, repoRoot string, res *pimalaya.Result, maxBody int, excludeCollections []string, events *hostevent.Bus) (mailreport.Artifact, bool, error) {
	const op = "cli.postSyncClassify"
	var art mailreport.Artifact
	if parent == nil {
		return art, false, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if res == nil || len(res.Raw) == 0 {
		return art, false, nil
	}
	rep := pimalaya.SummarizeNeverestJSON(res.Raw)
	if !continueClassifyAfterSync(rep) {
		return art, false, nil
	}
	reader, err := pimdir.OpenStore(cfg.Pimalaya.PimdirPath)
	if err != nil {
		return art, false, sirerr.Wrap(err, sirerr.CodeFailed, op, "open store")
	}
	ctx, cancel, err := classifyJobContext(parent, false)
	if err != nil {
		return art, false, err
	}
	defer cancel()
	items, unresolved, source, err := postSyncClassifyItems(ctx, reader, res, rep, maxBody, excludeCollections, cfg.Taxonomy.Collections)
	if err != nil {
		return art, false, err
	}
	runner, err := prepareClassifyRunner(ctx, cfg, repoRoot, classifyOpts{maxBody: maxBody, Events: events})
	if down, err := classifyUnavailable(err); down || err != nil {
		return art, down, err
	}
	defer func() { _ = runner.Close() }()
	items = mailreport.DedupeItemsByHash(items)
	art, err = runner.Classify(ctx, source, items)
	if down, err := classifyUnavailable(err); down || err != nil {
		return art, down, err
	}
	art.Unresolved += unresolved
	art, err = runner.ScoreAttention(ctx, art)
	if down, err := classifyUnavailable(err); down || err != nil {
		return art, down, err
	}
	art, err = runner.Cluster(ctx, art)
	if down, err := classifyUnavailable(err); down || err != nil {
		return art, down, err
	}
	if runner.WorthReadPriorEnabled() {
		art, err = runner.ScoreWorthReadPrior(ctx, art)
		if down, err := classifyUnavailable(err); down || err != nil {
			return art, down, err
		}
	}
	path, werr := writeReportArtifact(repoRoot, "", art)
	if werr != nil {
		return art, false, sirerr.Wrap(werr, sirerr.CodeFailed, op, "write report")
	}
	art.ReportPath = path
	return art, false, nil
}

func prepareClassifyRunner(ctx context.Context, cfg config.Config, repoRoot string, cOpts classifyOpts) (*mailreport.Runner, error) {
	const op = "cli.prepareClassifyRunner"
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, classifyJobTimeout)
		defer cancel()
	}
	return mailreport.PrepareRunner(ctx, cfg, mailreport.PrepareOpts{
		RepoRoot:        repoRoot,
		CatalogPath:     cOpts.catalogPath,
		NoApply:         cOpts.noApply,
		SkipAuthorProbe: cOpts.skipAuthorProbe,
		SkipEmbedProbe:  cOpts.skipEmbedProbe,
		Events:          cOpts.Events,
		Now:             time.Now().UTC(),
	})
}

func isClassifyUnavailable(err error) bool {
	code, ok := sirerr.AsCode(err)
	return ok && code == sirerr.CodeUnavailable
}

func classifyUnavailable(err error) (down bool, retErr error) {
	if err == nil {
		return false, nil
	}
	if isClassifyUnavailable(err) {
		return true, nil
	}
	return false, err
}

func shortClassifyErr(err error) string {
	if err == nil {
		return ""
	}
	s := strings.TrimSpace(err.Error())
	if len(s) > 160 {
		return s[:160] + "..."
	}
	return s
}

func itemsFromFetchHunks(ctx context.Context, reader *pimdir.Reader, hunks []pimalaya.HunkRef, maxBody int, excludeCollections []string) ([]mailreport.Item, int) {
	var items []mailreport.Item
	unresolved := 0
	for _, h := range hunks {
		if pimalaya.CollectionExcluded(h.Collection, excludeCollections) {
			continue
		}
		sum, err := reader.FindByIMAPRef(ctx, h.Collection, h.ID)
		if err != nil {
			unresolved++
			continue
		}
		if strings.TrimSpace(sum.ObjectHash) == "" {
			unresolved++
			continue
		}
		items = append(items, itemFromSummary(reader, sum, maxBody))
	}
	return items, unresolved
}

func itemFromSummary(reader *pimdir.Reader, sum pimdir.EmailSummary, maxBody int) mailreport.Item {
	hash := strings.TrimSpace(sum.ObjectHash)
	item := mailreport.Item{
		ObjectHash: hash,
		Subject:    sum.Subject,
		Sender:     senderLine(sum),
		Collection: sum.Collection,
	}
	if hash == "" {
		return item
	}
	if reader != nil {
		if blob, err := reader.ReadBlob(hash); err == nil {
			if prev, err := pimdir.DecodeMessagePreview(blob, maxBody); err == nil {
				item.Body = prev.Body
			}
			if hdr, err := pimdir.ParseMailHeaders(blob); err == nil {
				item.Headers = hdr
			}
		}
	}
	if strings.TrimSpace(item.Body) == "" {
		item.Body = truncateExportBody(sum.Body, maxBody)
	}
	return item
}

func truncateExportBody(body string, maxBody int) string {
	body = strings.TrimSpace(body)
	if maxBody > 0 && len(body) > maxBody {
		return body[:maxBody]
	}
	return body
}

func validateMailReportWindow(inPath, fromDate, toDate string) (pimdir.DateWindow, error) {
	const op = "cli.validateMailReportWindow"
	fromDate = strings.TrimSpace(fromDate)
	toDate = strings.TrimSpace(toDate)
	if strings.TrimSpace(inPath) != "" && (fromDate != "" || toDate != "") {
		return pimdir.DateWindow{}, sirerr.New(sirerr.CodeInvalid, op, "--in cannot be used with --from or --to")
	}
	if fromDate == "" && toDate == "" {
		return pimdir.DateWindow{}, nil
	}
	window, err := pimdir.CreateDateWindow(fromDate, toDate)
	if err != nil {
		return pimdir.DateWindow{}, wrapCLIHop(err, op, "parse --from/--to window")
	}
	return window, nil
}

func reportListLimit(limit int, limitChanged bool, window pimdir.DateWindow) int {
	if window.Zero() {
		return limit
	}
	if limitChanged {
		return limit
	}
	return 0
}

func loadReportItems(ctx context.Context, reader *pimdir.Reader, inPath string, window pimdir.DateWindow, limit int, limitChanged bool, maxBody int, collections []string) ([]mailreport.Item, error) {
	if strings.TrimSpace(inPath) != "" {
		return itemsFromExportFile(reader, inPath, maxBody)
	}
	listLimit := reportListLimit(limit, limitChanged, window)
	var rows []pimdir.EmailSummary
	var err error
	if !window.Zero() {
		rows, err = reader.ListEmailsIn(ctx, collections, window, listLimit)
	} else {
		rows, err = reader.ListRecentEmailsIn(ctx, collections, listLimit)
	}
	if err != nil {
		return nil, wrapCLIHop(err, "cli.loadReportItems", "list mail")
	}
	var items []mailreport.Item
	for _, sum := range rows {
		items = append(items, itemFromSummary(reader, sum, maxBody))
	}
	return items, nil
}

type exportEnvelope struct {
	Emails []pimdir.EmailSummary `json:"emails"`
}

func itemsFromExportFile(reader *pimdir.Reader, path string, maxBody int) ([]mailreport.Item, error) {
	const op = "cli.itemsFromExportFile"
	//nolint:gosec // operator-supplied mail export path (--in)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "read export")
	}
	var env exportEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode export")
	}
	var items []mailreport.Item
	for _, sum := range env.Emails {
		if strings.TrimSpace(sum.ObjectHash) == "" {
			continue
		}
		items = append(items, itemFromSummary(reader, sum, maxBody))
	}
	return items, nil
}

func senderLine(s pimdir.EmailSummary) string {
	name := strings.TrimSpace(s.SenderName)
	addr := strings.TrimSpace(s.Sender)
	if name != "" && addr != "" {
		return name + " <" + addr + ">"
	}
	if name != "" {
		return name
	}
	return addr
}

func writeReportArtifact(repoRoot, outPath string, art mailreport.Artifact) (string, error) {
	const op = "cli.writeReportArtifact"
	out := strings.TrimSpace(outPath)
	if out == "" {
		stamp, err := reportFileStamp(art.GeneratedAt)
		if err != nil {
			return "", wrapCLIHop(err, op, "report stamp")
		}
		tmpDir := filepath.Join(repoRoot, "tmp")
		if err := os.MkdirAll(tmpDir, 0o750); err != nil {
			return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir tmp")
		}
		out = filepath.Join(tmpDir, fmt.Sprintf("unwanted-report-%s.json", stamp))
	}
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "encode report")
	}
	if err := os.WriteFile(out, append(raw, '\n'), 0o600); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "write report")
	}
	return out, nil
}

const (
	humanLabelYes = "yes"
	humanLabelNo  = "no"
)

func humanBoolLabel(v bool) string {
	if v {
		return humanLabelYes
	}
	return humanLabelNo
}

func hasEssenceParseError(art mailreport.Artifact) bool {
	return essenceParseErrorCount(art) > 0
}

func essenceParseErrorCount(art mailreport.Artifact) int {
	n := 0
	for _, m := range art.Messages {
		if strings.Contains(m.Error, "essence parse") {
			n++
		}
	}
	return n
}

func classifyFailedMessages(art mailreport.Artifact) []mailreport.MessageRow {
	var failed []mailreport.MessageRow
	for _, m := range art.Messages {
		if strings.TrimSpace(m.Error) != "" {
			failed = append(failed, m)
		}
	}
	return failed
}

func classifyAuthorModel(ctx context.Context, prober mailreport.ChatProber, catalogIDs []string, authorYAML string, skipProbe bool) (string, error) {
	if skipProbe {
		return mailreport.PickAuthorModel(catalogIDs, authorYAML)
	}
	model, _, err := mailreport.ResolveAuthorModel(ctx, prober, catalogIDs, authorYAML)
	return model, err
}

func glossClassifyMsgErr(errMsg string) string {
	return mailreport.ClassifyFailureCause(errMsg)
}

func truncateClassifySubject(subject string, max int) string {
	return truncateRunes(subject, max)
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func formatClassifyTLDR(tldr string) string {
	const indent = "    "
	if strings.TrimSpace(tldr) == "" {
		return ""
	}
	wrapped := clui.WrapToWidth(tldr, clui.BoxContentWidth()-len(indent))
	var b strings.Builder
	for i, line := range strings.Split(wrapped, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(indent)
		b.WriteString(line)
	}
	return b.String()
}

func jobContextIncomplete(ctx context.Context) string {
	if ctx == nil || ctx.Err() == nil {
		return ""
	}
	return ctx.Err().Error()
}

func classifyReportWindowSlice(
	cmdCtx context.Context,
	runner *mailreport.Runner,
	reader *pimdir.Reader,
	inPath string,
	sw pimdir.DateWindow,
	limit int,
	limitChanged bool,
	maxBody int,
	collections []string,
) (mailreport.Artifact, error) {
	sliceCtx, cancel, err := classifyJobContext(cmdCtx, true)
	if err != nil {
		return mailreport.Artifact{}, err
	}
	defer cancel()
	items, err := loadReportItems(sliceCtx, reader, inPath, sw, limit, limitChanged, maxBody, collections)
	if err != nil {
		return mailreport.Artifact{}, err
	}
	items = mailreport.DedupeItemsByHash(items)
	sliceArt, err := runner.Classify(sliceCtx, mailreport.SourceReport, items)
	if err != nil {
		return mailreport.Artifact{}, err
	}
	sliceArt.IncompleteError = jobContextIncomplete(sliceCtx)
	return sliceArt, nil
}

func classifyJobContext(parent context.Context, windowed bool) (context.Context, context.CancelFunc, error) {
	const op = "cli.classifyJobContext"
	if parent == nil {
		return nil, func() {}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	timeout := classifyJobTimeout
	if windowed {
		timeout = classifyWindowJobTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	return ctx, cancel, nil
}

func wrapCLIHop(err error, op, msg string) error {
	if err == nil {
		return nil
	}
	code := sirerr.CodeFailed
	if c, ok := sirerr.AsCode(err); ok && c == sirerr.CodeInvalid {
		code = sirerr.CodeInvalid
	}
	return sirerr.Wrap(err, code, op, msg)
}

func reportFileStamp(generatedAt string) (string, error) {
	const op = "cli.reportFileStamp"
	raw := strings.TrimSpace(generatedAt)
	if raw == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "generated_at is empty")
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeInvalid, op, "parse generated_at")
	}
	return t.UTC().Format("20060102T150405Z"), nil
}

func formatClassifyArtifact(art mailreport.Artifact) string {
	var b strings.Builder
	writeClassifyCounts(&b, art)
	writeClassifyFailures(&b, art)
	writeClassifyClusters(&b, art)
	if art.ReportPath != "" {
		b.WriteString(art.ReportPath)
	}
	return clui.FormatBox("Classify", strings.TrimRight(b.String(), "\n"))
}

func writeClassifyCounts(b *strings.Builder, art mailreport.Artifact) {
	if art.WindowFrom != "" && art.WindowTo != "" {
		fmt.Fprintf(b, "Window [%s, %s) UTC\n", art.WindowFrom, art.WindowTo)
	}
	failed := classifyFailedMessages(art)
	fmt.Fprintf(b, "Classified %d", art.Classified)
	if len(failed) > 0 {
		fmt.Fprintf(b, " · failed %d", len(failed))
	}
	b.WriteByte('\n')
	if art.ShapeConfirmed > 0 {
		fmt.Fprintf(b, "shape-confirmed %d (Jev, skipped Essence)\n", art.ShapeConfirmed)
	}
	if art.ShapesPromoted > 0 {
		fmt.Fprintf(b, "shapes promoted %d\n", art.ShapesPromoted)
	}
	if art.Omitted > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d omitted (cap)\n", art.Omitted)))
	}
	if art.AttentionShapeReused > 0 {
		fmt.Fprintf(b, "attention shape-reused %d (skipped SystemOne)\n", art.AttentionShapeReused)
	}
	if art.AttentionOmitted > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d attention omitted (cap)\n", art.AttentionOmitted)))
	}
	if art.Unresolved > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d unresolved hunks\n", art.Unresolved)))
	}
	if art.JudgeModel != "" || art.AuthorModel != "" {
		fmt.Fprintf(b, "judge %s · author %s\n", art.JudgeModel, art.AuthorModel)
	}
	fmt.Fprintf(b, "aliases added %d · new leaves %d\n", art.AliasesAdded, art.LeavesAdded)
	embedMiss := 0
	for _, m := range art.Messages {
		if strings.TrimSpace(m.EmbedPersist) != "" {
			embedMiss++
		}
	}
	if embedMiss > 0 {
		b.WriteString(clui.Muted(fmt.Sprintf("%d classified without stored embedding\n", embedMiss)))
	}
	if art.EssenceDumpDir != "" && hasEssenceParseError(art) {
		n := essenceParseErrorCount(art)
		b.WriteString(clui.Muted(fmt.Sprintf("essence dumps %d · %s\n", n, art.EssenceDumpDir)))
	}
}

func writeClassifyFailures(b *strings.Builder, art mailreport.Artifact) {
	failed := classifyFailedMessages(art)
	if len(failed) == 0 {
		return
	}
	fmt.Fprintf(b, "\nFailed %d\n", len(failed))
	for _, m := range failed {
		label := truncateClassifySubject(m.Subject, 56)
		if label == "" {
			label = m.ObjectHash
		}
		fmt.Fprintf(b, "  %s  %s\n", label, glossClassifyMsgErr(m.Error))
	}
}

func writeClassifyClusters(b *strings.Builder, art mailreport.Artifact) {
	if len(art.Clusters) == 0 {
		return
	}
	fmt.Fprintf(b, "%d clusters\n", len(art.Clusters))
	for _, c := range art.Clusters {
		writeClassifyClusterLine(b, c)
	}
}

func writeClassifyClusterLine(b *strings.Builder, c mailreport.ClusterRow) {
	label := c.TermID
	if label == "" && c.SenderDomain != "" {
		label = c.SenderDomain
	}
	if label == "" {
		label = c.ID
	}
	tldr := strings.TrimSpace(c.TLDR)
	if tldr == "" && c.TLDRError != "" {
		tldr = clui.Muted("(no tldr)")
	}
	if c.Attention != nil {
		fmt.Fprintf(b, "  %s (%d) %s  urg=%.2f reply=%s decision=%s\n",
			label, c.Size, c.SuggestedAction, c.Attention.Urgency,
			humanBoolLabel(c.Attention.NeedsReply), humanBoolLabel(c.Attention.InvolvesDecision))
	} else {
		fmt.Fprintf(b, "  %s (%d) %s\n", label, c.Size, c.SuggestedAction)
	}
	if tldrLine := formatClassifyTLDR(tldr); tldrLine != "" {
		b.WriteString(tldrLine)
		b.WriteByte('\n')
	}
}
