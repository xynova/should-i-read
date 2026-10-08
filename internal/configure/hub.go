package configure

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// RunHub runs the TTY hub loop until quit.
func RunHub(ctx context.Context, r *Runner, prompter HubPrompter, opts Options, out, errW io.Writer) (SessionResult, error) {
	const op = "configure.RunHub"
	if ctx == nil {
		return SessionResult{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if r == nil || prompter == nil {
		return SessionResult{}, sirerr.New(sirerr.CodeInvalid, op, "nil runner or prompter")
	}
	if out == nil {
		out = io.Discard
	}
	if errW == nil {
		errW = io.Discard
	}
	opts.Interactive = true
	session := SessionResult{Ran: []StepResult{}}
	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return session, sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
		}
		cfg, err := config.Load(r.repoRoot, "")
		if err != nil {
			return session, err
		}
		snap, err := CollectSnapshot(ctx, r.repoRoot, cfg)
		if err != nil {
			return session, err
		}
		session.Snapshot = snap
		applyMailFromSnapshot(&opts, snap)

		ClearScreen(out)
		fmt.Fprintln(out, FormatChecklist(snap))
		if lastErr != nil {
			fmt.Fprintln(out, FormatStepFailed(lastErr))
			fmt.Fprintln(out)
			lastErr = nil
		}

		action, err := prompter.ChooseAction(ctx, snap)
		if err != nil {
			return session, err
		}
		switch action {
		case HubQuit:
			fmt.Fprintln(out, FormatHubSummary(snap))
			return session, nil
		case HubRunMissing:
			subOpts := opts
			subOpts.Interactive = true
			applyMailFromSnapshot(&subOpts, snap)
			fmt.Fprintln(out, clui.Muted("Running missing steps…"))
			if email := strings.TrimSpace(subOpts.Email); email != "" {
				fmt.Fprintln(out, clui.Muted("Using saved mailbox "+email))
			}
			res, err := r.RunAllMissing(ctx, subOpts, out, errW, prompter.ConfirmBrowserLogin)
			session.Ran = append(session.Ran, res.Ran...)
			session.Snapshot = res.Snapshot
			if err != nil {
				lastErr = err
				continue
			}
		case HubFixStep:
			stepID, err := prompter.ChooseStep(ctx, snap)
			if err != nil {
				return session, err
			}
			subOpts := opts
			applyMailFromSnapshot(&subOpts, snap)
			if needsMailboxPrompt(stepID, subOpts, snap) {
				prefill := MailboxPrefill{Provider: subOpts.Provider, Email: subOpts.Email, Account: subOpts.Account}
				prefill, err = prompter.CollectMailboxFields(ctx, prefill)
				if err != nil {
					return session, err
				}
				subOpts.Provider = prefill.Provider
				subOpts.Email = prefill.Email
				subOpts.Account = prefill.Account
				opts.Provider = subOpts.Provider
				opts.Email = subOpts.Email
				opts.Account = subOpts.Account
			} else if needsMailFields(stepID) && strings.TrimSpace(subOpts.Email) != "" {
				fmt.Fprintln(out, clui.Muted("Using saved mailbox "+strings.TrimSpace(subOpts.Email)))
			}
			fmt.Fprintf(out, "%s\n", clui.Muted("→ "+StepTitle(stepID)+"…"))
			stepRes, _, err := r.RunStep(ctx, stepID, subOpts, out, errW, true)
			session.Ran = append(session.Ran, stepRes)
			if err != nil {
				lastErr = err
				continue
			}
		default:
			return session, sirerr.New(sirerr.CodeInvalid, op, "unknown hub action")
		}
	}
}
