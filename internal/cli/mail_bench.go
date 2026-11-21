package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/mailreport"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type mailBenchRunFlags struct {
	variant   string
	chunksDir string
	ledgers   string
	outPath   string
	scorePath string
	labels    string
	revPath   string
	maxBody   int
	fresh     bool
}

func newMailBenchCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	run := mailBenchRunFlags{maxBody: 8192}
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Essence benchmark freeze and variant runs (report-only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(run.variant) == "" {
				return cmd.Help()
			}
			return runMailBenchVariant(cmd, opts, repoRoot, run)
		},
	}
	addMailBenchRunFlags(cmd, &run)
	cmd.AddCommand(newMailBenchFreezeCmd(opts, repoRoot))
	cmd.AddCommand(newMailBenchLabelsCmd(opts, repoRoot))
	return cmd
}

func addMailBenchRunFlags(cmd *cobra.Command, run *mailBenchRunFlags) {
	cmd.Flags().StringVar(&run.variant, "variant", "", "baseline|thinking|checklist")
	cmd.Flags().StringVar(&run.chunksDir, "chunks", "", "Chunk directory (default bench/chunks)")
	cmd.Flags().StringVar(&run.ledgers, "ledgers", "", "Per-chunk ledger directory")
	cmd.Flags().StringVar(&run.outPath, "out", "", "Merged artifact JSON path")
	cmd.Flags().StringVar(&run.scorePath, "score-out", "", "Scorecard JSON path")
	cmd.Flags().StringVar(&run.labels, "labels", "", "labels-20.json path")
	cmd.Flags().StringVar(&run.revPath, "input-rev", "", "input-rev.txt path")
	cmd.Flags().IntVar(&run.maxBody, "max-body", 8192, "Max body bytes per message")
	cmd.Flags().BoolVar(&run.fresh, "fresh", false, "Remove per-variant ledger under tmp/bench/<variant> before classify")
}

func newMailBenchFreezeCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		inPath string
		outDir string
		gitSHA string
	)
	cmd := &cobra.Command{
		Use:   "freeze",
		Short: "Freeze export JSON into input-100 and chunk files",
		RunE: func(cmd *cobra.Command, args []string) error {
			const op = "cli.mail.bench.freeze"
			if strings.TrimSpace(inPath) == "" {
				return sirerr.New(sirerr.CodeInvalid, op, "--in export path required")
			}
			dir := strings.TrimSpace(outDir)
			if dir == "" {
				dir = filepath.Join(repoRoot, "bench")
			}
			sha := strings.TrimSpace(gitSHA)
			if sha == "" {
				sha = gitHeadSHA(repoRoot)
			}
			res, err := mailreport.FreezeBenchInput(inPath, dir, sha)
			if err != nil {
				return wrapCLIHop(err, op, "freeze bench input")
			}
			if wantJSON(cmd) {
				return printJSON(cmd.OutOrStdout(), map[string]string{
					"input":    res.InputPath,
					"revision": res.RevisionPath,
					"chunks":   res.ChunksDir,
					"sha256":   res.ContentSHA,
				})
			}
			return writeCLILine(cmd.OutOrStdout(), fmt.Sprintf("Frozen bench input\n%s\n%s\n", res.InputPath, res.ChunksDir))
		},
	}
	cmd.Flags().StringVar(&inPath, "in", "", "Mail export JSON path")
	cmd.Flags().StringVar(&outDir, "out-dir", "", "Output directory (default bench/)")
	cmd.Flags().StringVar(&gitSHA, "git-sha", "", "Git SHA for revision stamp (default HEAD)")
	return cmd
}

func newMailBenchLabelsCmd(_ *rootOptions, repoRoot string) *cobra.Command {
	var (
		inputPath string
		revPath   string
		outPath   string
	)
	cmd := &cobra.Command{
		Use:   "labels-stub",
		Short: "Write labels-20.json stub for human labeling",
		RunE: func(cmd *cobra.Command, args []string) error {
			const op = "cli.mail.bench.labels-stub"
			in := strings.TrimSpace(inputPath)
			if in == "" {
				in = filepath.Join(repoRoot, "bench", "input-100.json")
			}
			revFile := strings.TrimSpace(revPath)
			if revFile == "" {
				revFile = filepath.Join(repoRoot, "bench", "input-rev.txt")
			}
			rev, err := mailreport.ReadInputRevision(revFile)
			if err != nil {
				return wrapCLIHop(err, op, "read revision")
			}
			out := strings.TrimSpace(outPath)
			if out == "" {
				out = filepath.Join(repoRoot, "bench", "labels-20.json")
			}
			if err := mailreport.WriteBenchLabelStub(in, rev, out); err != nil {
				return wrapCLIHop(err, op, "write label stub")
			}
			if wantJSON(cmd) {
				return printJSON(cmd.OutOrStdout(), map[string]string{"labels": out, "revision": rev})
			}
			return writeCLILine(cmd.OutOrStdout(), fmt.Sprintf("Wrote label stub %s\n", out))
		},
	}
	cmd.Flags().StringVar(&inputPath, "input", "", "Frozen input-100.json path")
	cmd.Flags().StringVar(&revPath, "input-rev", "", "input-rev.txt path")
	cmd.Flags().StringVar(&outPath, "out", "", "labels-20.json output path")
	return cmd
}

func runMailBenchVariant(cmd *cobra.Command, opts *rootOptions, repoRoot string, run mailBenchRunFlags) error {
	const op = "cli.mail.bench"
	variant := strings.TrimSpace(run.variant)
	if err := mailreport.ApplyBenchVariant(variant); err != nil {
		return wrapCLIHop(err, op, "apply variant")
	}
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	in, err := loadBenchInputs(repoRoot, variant, run)
	if err != nil {
		return err
	}
	reader, storeErr := pimdir.OpenStore(cfg.Pimalaya.PimdirPath)
	if storeErr != nil {
		reader = nil
	}
	wallStart := time.Now()
	cOpts := classifyOpts{noApply: true, maxBody: run.maxBody, skipAuthorProbe: true, skipEmbedProbe: true}
	merged, chunkRuntimes, err := classifyBenchChunks(cmd.Context(), cfg, repoRoot, reader, storeErr, in.chunkFiles, in.ledgerRoot, cOpts)
	if err != nil {
		return err
	}
	merged.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	merged.PolypusBaseURL = cfg.PolypusBaseURL
	out := strings.TrimSpace(run.outPath)
	if out == "" {
		out = filepath.Join(repoRoot, "tmp", "bench", variant+"-artifact.json")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir artifact dir")
	}
	artPath, err := writeReportArtifact(repoRoot, out, merged)
	if err != nil {
		return wrapCLIHop(err, op, "write artifact")
	}
	merged.ReportPath = artPath
	prompts := mailreport.BuildEssencePrompts(nil)
	promptHash := mailreport.EssencePromptsHash(prompts)
	score := mailreport.ScoreBenchArtifact(
		merged,
		variant,
		in.inputRev,
		in.labelFile,
		promptHash,
		cfg.Taxonomy.DescriptionCosineMin,
		chunkRuntimes,
		time.Since(wallStart),
	)
	return emitBenchScore(cmd, repoRoot, variant, run, artPath, score)
}

type benchInputs struct {
	chunkFiles []string
	ledgerRoot string
	inputRev   string
	labelFile  mailreport.BenchLabelsFile
}

func loadBenchInputs(repoRoot, variant string, run mailBenchRunFlags) (benchInputs, error) {
	const op = "cli.mail.bench"
	chunkDir := strings.TrimSpace(run.chunksDir)
	if chunkDir == "" {
		chunkDir = filepath.Join(repoRoot, "bench", "chunks")
	}
	chunkFiles, err := mailreport.ListBenchChunkFiles(chunkDir)
	if err != nil {
		return benchInputs{}, wrapCLIHop(err, op, "list chunks")
	}
	revFile := strings.TrimSpace(run.revPath)
	if revFile == "" {
		revFile = filepath.Join(repoRoot, "bench", "input-rev.txt")
	}
	inputRev, err := mailreport.ReadInputRevision(revFile)
	if err != nil {
		return benchInputs{}, wrapCLIHop(err, op, "read input revision")
	}
	labelPath := strings.TrimSpace(run.labels)
	if labelPath == "" {
		labelPath = filepath.Join(repoRoot, "bench", "labels-20.json")
	}
	labelFile, err := mailreport.LoadBenchLabels(labelPath, inputRev)
	if err != nil {
		return benchInputs{}, wrapCLIHop(err, op, "load labels")
	}
	ledgerRoot := strings.TrimSpace(run.ledgers)
	if ledgerRoot == "" {
		ledgerRoot = filepath.Join(repoRoot, "tmp", "bench", variant)
	}
	if run.fresh {
		if err := os.RemoveAll(ledgerRoot); err != nil {
			return benchInputs{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "remove ledger dir").With("path", ledgerRoot)
		}
	}
	if err := os.MkdirAll(ledgerRoot, 0o750); err != nil {
		return benchInputs{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir ledgers")
	}
	return benchInputs{chunkFiles: chunkFiles, ledgerRoot: ledgerRoot, inputRev: inputRev, labelFile: labelFile}, nil
}

func emitBenchScore(cmd *cobra.Command, repoRoot, variant string, run mailBenchRunFlags, artPath string, score mailreport.BenchScore) error {
	const op = "cli.mail.bench"
	scoreOut := strings.TrimSpace(run.scorePath)
	if scoreOut == "" {
		scoreOut = filepath.Join(repoRoot, "tmp", "bench", variant+"-score.json")
	}
	if err := os.MkdirAll(filepath.Dir(scoreOut), 0o750); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir score dir")
	}
	if err := mailreport.WriteBenchScore(scoreOut, score); err != nil {
		return wrapCLIHop(err, op, "write score")
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), map[string]any{
			"artifact": artPath,
			"score":    scoreOut,
			"variant":  variant,
		})
	}
	return writeCLILine(cmd.OutOrStdout(), fmt.Sprintf("Bench %s classified=%d failed=%d\n%s\n%s\n",
		variant, score.Classified, score.Failed, artPath, scoreOut))
}

func classifyBenchChunks(ctx context.Context, cfg config.Config, repoRoot string, reader *pimdir.Reader, storeErr error, chunkFiles []string, ledgerRoot string, cOpts classifyOpts) (mailreport.Artifact, map[string]time.Duration, error) {
	merged := mailreport.Artifact{Mode: mailreport.ModeReportOnly, Source: mailreport.SourceBench}
	chunkRuntimes := map[string]time.Duration{}
	for i, chunkFile := range chunkFiles {
		chunkID := filepath.Base(chunkFile)
		chunkStart := time.Now()
		sliceArt, err := classifyOneBenchChunk(ctx, cfg, repoRoot, reader, storeErr, chunkFile, ledgerRoot, i, cOpts)
		if err != nil {
			return merged, nil, err
		}
		mailreport.MergeClassifyArtifacts(&merged, sliceArt)
		chunkRuntimes[chunkID] = time.Since(chunkStart)
	}
	return merged, chunkRuntimes, nil
}

func classifyOneBenchChunk(ctx context.Context, cfg config.Config, repoRoot string, reader *pimdir.Reader, storeErr error, chunkFile, ledgerRoot string, i int, cOpts classifyOpts) (mailreport.Artifact, error) {
	const op = "cli.mail.bench"
	chunkID := filepath.Base(chunkFile)
	chunkCtx, cancel, err := classifyJobContext(ctx, false)
	if err != nil {
		return mailreport.Artifact{}, wrapCLIHop(err, op, "chunk context")
	}
	defer cancel()
	chunkCfg := cfg
	chunkCfg.StorePath = filepath.Join(ledgerRoot, fmtChunkLedger(i))
	chunkCfg.Taxonomy.ShapeConfirm = false
	runner, err := prepareClassifyRunner(chunkCtx, chunkCfg, repoRoot, cOpts)
	if err != nil {
		return mailreport.Artifact{}, wrapCLIHop(err, op, "prepare runner "+chunkID)
	}
	items, err := loadReportItems(chunkCtx, reader, chunkFile, pimdir.DateWindow{}, 0, false, cOpts.maxBody, chunkCfg.Taxonomy.Collections)
	if err != nil {
		_ = runner.Close()
		return mailreport.Artifact{}, wrapCLIHop(err, op, "load chunk items "+chunkID)
	}
	items = mailreport.DedupeItemsByHash(items)
	if err := requireItemBodies(items, storeErr); err != nil {
		_ = runner.Close()
		return mailreport.Artifact{}, wrapCLIHop(err, op, "chunk bodies "+chunkID)
	}
	sliceArt, err := runner.Classify(chunkCtx, mailreport.SourceBench, items)
	closeErr := runner.Close()
	if err != nil {
		return mailreport.Artifact{}, wrapCLIHop(err, op, "classify chunk "+chunkID)
	}
	if closeErr != nil {
		return mailreport.Artifact{}, wrapCLIHop(closeErr, op, "close runner "+chunkID)
	}
	return sliceArt, nil
}

func fmtChunkLedger(i int) string {
	return fmt.Sprintf("c%d.db", i+1)
}

func gitHeadSHA(repoRoot string) string {
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD").Output() //nolint:gosec // G204: git argv is fixed; repoRoot is the operator checkout.
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func requireItemBodies(items []mailreport.Item, storeErr error) error {
	const op = "cli.requireItemBodies"
	for _, it := range items {
		if strings.TrimSpace(it.Body) != "" {
			continue
		}
		if storeErr != nil {
			return sirerr.Wrap(storeErr, sirerr.CodeFailed, op, "pimdir required when export body is empty").
				With("object_hash", it.ObjectHash)
		}
		return sirerr.New(sirerr.CodeInvalid, op, "empty message body").With("object_hash", it.ObjectHash)
	}
	return nil
}
