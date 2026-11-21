package mailjob

import (
	"context"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

func (r *Runner) refreshBindings(ctx context.Context, jobID string, items []Item) error {
	const op = "mailjob.Runner.refreshBindings"
	if r == nil || r.resolver == nil || len(items) == 0 {
		return nil
	}
	hashes := make([]string, 0, len(items))
	for _, it := range items {
		hashes = append(hashes, it.ObjectHash)
	}
	bindings, err := r.resolver.Resolve(ctx, hashes)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "resolve job bindings")
	}
	out := make([]Binding, 0, len(bindings))
	for _, b := range bindings {
		if b.Status != r.spec.PlannedStatus && b.Status != r.spec.RefusedStatus {
			continue
		}
		if strings.TrimSpace(b.Folder) == "" && b.Status == r.spec.PlannedStatus {
			continue
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return nil
	}
	if err := r.store.BindItems(ctx, jobID, out); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "bind refreshed items")
	}
	return nil
}

func (r *Runner) resolveItem(ctx context.Context, jobID string, it Item) (folder, env string, skip bool, err error) {
	const op = "mailjob.Runner.resolveItem"
	folder = Mailbox(it)
	env = EnvelopeID(it)
	if folder != "" && env != "" {
		return folder, env, false, nil
	}
	bindings, rerr := r.resolver.Resolve(ctx, []string{it.ObjectHash})
	if rerr != nil || len(bindings) == 0 {
		if serr := r.store.SetItemStatus(ctx, jobID, it.ObjectHash, r.spec.UnresolvedStatus, "re-resolve failed"); serr != nil {
			return "", "", true, sirerr.Wrap(serr, sirerr.CodeFailed, op, "set unresolved after re-resolve")
		}
		return "", "", true, nil
	}
	if err := r.store.BindItems(ctx, jobID, bindings); err != nil {
		return "", "", true, sirerr.Wrap(err, sirerr.CodeFailed, op, "bind re-resolved item")
	}
	folder = bindings[0].Folder
	env = bindings[0].EnvelopeID
	if bindings[0].Status != r.spec.PlannedStatus {
		return "", "", true, nil
	}
	return folder, env, false, nil
}
