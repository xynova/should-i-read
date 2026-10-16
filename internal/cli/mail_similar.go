package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/mailsync"
	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const hostStoreOpTimeout = 30 * time.Second

func newMailSimilarCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	var k int
	cmd := &cobra.Command{
		Use:   "similar <object_hash>",
		Short: "Nearest stored embeddings for one classified message",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMailSimilar(cmd, opts, repoRoot, args[0], k)
		},
	}
	cmd.Flags().IntVar(&k, "k", 5, "Max neighbors (1-50)")
	return cmd
}

type similarNeighborJSON struct {
	ObjectHash string  `json:"object_hash"`
	Cosine     float64 `json:"cosine"`
	Subject    string  `json:"subject,omitempty"`
	Sender     string  `json:"sender,omitempty"`
}

type similarJSON struct {
	ObjectHash string                `json:"object_hash"`
	ModelID    string                `json:"model_id"`
	K          int                   `json:"k"`
	Neighbors  []similarNeighborJSON `json:"neighbors"`
}

func runMailSimilar(cmd *cobra.Command, opts *rootOptions, repoRoot, hash string, k int) error {
	const op = "cli.mail.similar"
	if k < 1 {
		return sirerr.New(sirerr.CodeInvalid, op, "k out of range")
	}
	if k > 50 {
		k = 50
	}
	cfg, err := opts.mustLoad(repoRoot)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), hostStoreOpTimeout)
	defer cancel()
	store, err := openHostStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	model := strings.TrimSpace(cfg.PolypusEmbedModel)
	hits, err := store.Similar(ctx, hash, model, k)
	if err != nil {
		return err
	}
	reader, _ := pimdir.OpenStore(cfg.Pimalaya.PimdirPath)
	out := similarJSON{ObjectHash: hash, ModelID: model, K: k, Neighbors: []similarNeighborJSON{}}
	for _, h := range hits {
		n := similarNeighborJSON{ObjectHash: h.ObjectHash, Cosine: h.Cosine}
		if reader != nil {
			if sum, findErr := reader.FindByObjectHash(h.ObjectHash); findErr == nil {
				n.Subject = sum.Subject
				n.Sender = sum.Sender
			}
		}
		out.Neighbors = append(out.Neighbors, n)
	}
	if wantJSON(cmd) {
		return printJSON(cmd.OutOrStdout(), out)
	}
	if len(out.Neighbors) == 0 {
		return writeCLILine(cmd.OutOrStdout(), clui.Muted("No neighbors"))
	}
	var b strings.Builder
	for _, n := range out.Neighbors {
		fmt.Fprintf(&b, "%.3f  %s  (%s)\n", n.Cosine, n.Subject, n.ObjectHash)
	}
	return writeCLILine(cmd.OutOrStdout(), strings.TrimRight(b.String(), "\n"))
}

func openHostStore(ctx context.Context, cfg config.Config) (*hoststore.Store, error) {
	const op = "cli.openHostStore"
	if ctx == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	now := time.Now().UTC()
	st, err := hoststore.CreateStore(hoststore.StoreConfig{Path: cfg.StorePath, Now: now})
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "open classify store")
	}
	return st, nil
}

func collectClassifyStoreStatus(ctx context.Context, cfg config.Config) mailsync.ClassifyStoreStatus {
	out := mailsync.ClassifyStoreStatus{Path: cfg.StorePath}
	opCtx, cancel := context.WithTimeout(ctx, hostStoreOpTimeout)
	defer cancel()
	store, err := hoststore.CreateStore(hoststore.StoreConfig{Path: cfg.StorePath, Now: time.Now().UTC()})
	if err != nil {
		return out
	}
	defer func() { _ = store.Close() }()
	c, err := store.Counts(opCtx)
	if err == nil {
		out.Complete = c.Complete
		out.Error = c.Error
		out.Running = c.Running
	}
	complete, _ := store.ListCompleteHashes(opCtx)
	done := map[string]struct{}{}
	for _, h := range complete {
		done[h] = struct{}{}
	}
	reader, err := pimdir.OpenStore(cfg.Pimalaya.PimdirPath)
	if err != nil {
		return out
	}
	rows, err := reader.ListRecentEmailsIn(opCtx, cfg.Taxonomy.Collections, 500)
	if err != nil {
		return out
	}
	pending := 0
	for _, r := range rows {
		h := strings.TrimSpace(r.ObjectHash)
		if h == "" {
			continue
		}
		if _, ok := done[h]; !ok {
			pending++
		}
	}
	out.Pending = pending
	return out
}
