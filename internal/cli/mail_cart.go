package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/maildelete"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func newMailCartCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cart",
		Short: "Deletion cart jobs (trash at source after confirm)",
	}
	cmd.AddCommand(newMailCartJobCmd(opts, repoRoot))
	cmd.AddCommand(newMailCartApplyCmd(opts, repoRoot))
	cmd.AddCommand(newMailCartStatusCmd(opts, repoRoot))
	return cmd
}

func newMailCartJobCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var ledger string
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Snapshot the cart, resolve hashes, write tmp/delete-job-<id>.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailCartJob(cmd, opts, repoRoot, ledger)
		},
	}
	cmd.Flags().StringVar(&ledger, "store", "", "Classify ledger path (default config store.path)")
	return cmd
}

func newMailCartApplyCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var (
		ledger    string
		confirm   string
		syncAfter bool
	)
	cmd := &cobra.Command{
		Use:   "apply <job-id>",
		Short: "Move planned items to Trash at the mail source (Himalaya)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailCartApply(cmd, opts, repoRoot, args[0], ledger, confirm, syncAfter)
		},
	}
	cmd.Flags().StringVar(&ledger, "store", "", "Classify ledger path (default config store.path)")
	cmd.Flags().StringVar(&confirm, "confirm", "", fmt.Sprintf("Required confirm token (%q)", hoststore.DeleteJobConfirmToken))
	cmd.Flags().BoolVar(&syncAfter, "sync-after", false, "Run mail sync after done or partial")
	return cmd
}

func newMailCartStatusCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var ledger string
	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Show deletion job JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailCartStatus(cmd, opts, repoRoot, args[0], ledger)
		},
	}
	cmd.Flags().StringVar(&ledger, "store", "", "Classify ledger path (default config store.path)")
	return cmd
}

func runMailCartJob(cmd *cobra.Command, opts *rootOptions, repoRoot, ledger string) error {
	const op = "cli.mail.cart.job"
	svc, cleanup, err := openDeleteJobService(cmd.Context(), opts, repoRoot, ledger)
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Minute)
	defer cancel()
	job, err := svc.StartJob(ctx)
	if err != nil {
		return wrapCLIHop(err, op, "start job")
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), job)
	}
	return writeCLILine(cmd.OutOrStdout(), formatDeleteJobSummary(job))
}

func runMailCartApply(cmd *cobra.Command, opts *rootOptions, repoRoot, jobID, ledger, confirm string, syncAfter bool) error {
	const op = "cli.mail.cart.apply"
	if strings.TrimSpace(confirm) == "" {
		return sirerr.New(sirerr.CodeInvalid, op, fmt.Sprintf("--confirm %q is required", hoststore.DeleteJobConfirmToken))
	}
	svc, cleanup, err := openDeleteJobService(cmd.Context(), opts, repoRoot, ledger)
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Hour)
	defer cancel()
	if err := svc.Apply(ctx, strings.TrimSpace(jobID), strings.TrimSpace(confirm)); err != nil {
		return wrapCLIHop(err, op, "apply job")
	}
	job, err := svc.GetJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return wrapCLIHop(err, op, "load job")
	}
	if syncAfter && (job.Status == hoststore.DeleteJobStatusDone || job.Status == hoststore.DeleteJobStatusPartial) {
		if err := runMailSync(cmd, opts, repoRoot, "", false, nil); err != nil {
			return wrapCLIHop(err, op, "sync after apply")
		}
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), job)
	}
	return writeCLILine(cmd.OutOrStdout(), formatDeleteJobSummary(job))
}

func runMailCartStatus(cmd *cobra.Command, opts *rootOptions, repoRoot, jobID, ledger string) error {
	const op = "cli.mail.cart.status"
	svc, cleanup, err := openDeleteJobService(cmd.Context(), opts, repoRoot, ledger)
	if err != nil {
		return err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(cmd.Context(), time.Minute)
	defer cancel()
	job, err := svc.GetJob(ctx, strings.TrimSpace(jobID))
	if err != nil {
		return wrapCLIHop(err, op, "get job")
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), job)
	}
	return writeCLILine(cmd.OutOrStdout(), formatDeleteJobSummary(job))
}

func openDeleteJobService(ctx context.Context, opts *rootOptions, repoRoot, ledgerPath string) (*maildelete.Service, func(), error) {
	const op = "cli.openDeleteJobService"
	if ctx == nil {
		return nil, nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return nil, nil, err
	}
	ledger := strings.TrimSpace(ledgerPath)
	if ledger == "" {
		ledger = cfg.StorePath
	}
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: ledger, Now: time.Now().UTC()})
	if err != nil {
		return nil, nil, wrapCLIHop(err, op, "open ledger")
	}
	reader, err := pimdir.OpenStore(cfg.Pimalaya.PimdirPath)
	if err != nil {
		_ = st.Close()
		return nil, nil, wrapCLIHop(err, op, "open pimdir")
	}
	var deleter maildelete.TrashDeleter
	h, herr := pimalaya.CreateHimalaya(cfg)
	if herr == nil && h != nil {
		deleter = h
	}
	svc, err := maildelete.CreateDeleteJobService(maildelete.DeleteJobConfig{
		Store:    st,
		Pimdir:   reader,
		Deleter:  deleter,
		Now:      time.Now,
		RepoRoot: repoRoot,
	})
	if err != nil {
		_ = st.Close()
		return nil, nil, wrapCLIHop(err, op, "create delete job service")
	}
	cleanup := func() {
		_ = st.Close()
	}
	return svc, cleanup, nil
}

func formatDeleteJobSummary(job maildelete.JobArtifact) string {
	return fmt.Sprintf("job %s status=%s planned=%d unresolved=%d refused=%d trashed=%d failed=%d",
		job.ID, job.Status,
		job.Counts.Planned, job.Counts.Unresolved, job.Counts.Refused,
		job.Counts.Trashed, job.Counts.Failed)
}
