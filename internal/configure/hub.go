package configure

import (
	"context"
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
		writeTTYLine(out, FormatChecklist(snap))
		if lastErr != nil {
			writeTTYLine(out, FormatStepFailed(lastErr))
			writeTTYLine(out, "")
		}

		action, err := prompter.ChooseAction(ctx, snap)
		if err != nil {
			return session, err
		}
		cont, actErr := applyHubAction(ctx, r, prompter, &opts, out, errW, &session, action, snap)
		if !cont {
			return session, actErr
		}
		lastErr = actErr
	}
}

func applyHubAction(ctx context.Context, r *Runner, prompter HubPrompter, opts *Options, out, errW io.Writer, session *SessionResult, action HubAction, snap Snapshot) (bool, error) {
	const op = "configure.RunHub"
	switch action {
	case HubQuit:
		writeTTYLine(out, FormatHubSummary(snap))
		return false, nil
	case HubRunMissing:
		return hubRunMissing(ctx, r, prompter, *opts, out, errW, session, snap)
	case HubFixStep:
		return hubFixStep(ctx, r, prompter, opts, out, errW, session, snap)
	default:
		return false, sirerr.New(sirerr.CodeInvalid, op, "unknown hub action")
	}
}

func hubRunMissing(ctx context.Context, r *Runner, prompter HubPrompter, opts Options, out, errW io.Writer, session *SessionResult, snap Snapshot) (bool, error) {
	opts.Interactive = true
	applyMailFromSnapshot(&opts, snap)
	writeTTYLine(out, clui.Muted("Running missing steps…"))
	if email := strings.TrimSpace(opts.Email); email != "" {
		writeTTYLine(out, clui.Muted("Using saved mailbox "+email))
	}
	res, err := r.RunAllMissing(ctx, opts, out, errW, prompter.ConfirmBrowserLogin)
	session.Ran = append(session.Ran, res.Ran...)
	session.Snapshot = res.Snapshot
	return true, err
}

func hubFixStep(ctx context.Context, r *Runner, prompter HubPrompter, opts *Options, out, errW io.Writer, session *SessionResult, snap Snapshot) (bool, error) {
	stepID, err := prompter.ChooseStep(ctx, snap)
	if err != nil {
		return false, err
	}
	subOpts := *opts
	applyMailFromSnapshot(&subOpts, snap)
	if err := promptMailboxIfNeeded(ctx, prompter, opts, &subOpts, snap, stepID, out); err != nil {
		return false, err
	}
	writeTTY(out, "%s\n", clui.Muted("→ "+StepTitle(stepID)+"…"))
	stepRes, _, err := r.RunStep(ctx, stepID, subOpts, out, errW, true)
	session.Ran = append(session.Ran, stepRes)
	return true, err
}

func promptMailboxIfNeeded(ctx context.Context, prompter HubPrompter, opts, subOpts *Options, snap Snapshot, stepID string, out io.Writer) error {
	if needsMailboxPrompt(stepID, *subOpts, snap) {
		prefill := MailboxPrefill{Provider: subOpts.Provider, Email: subOpts.Email, Account: subOpts.Account}
		prefill, err := prompter.CollectMailboxFields(ctx, prefill)
		if err != nil {
			return err
		}
		subOpts.Provider = prefill.Provider
		subOpts.Email = prefill.Email
		subOpts.Account = prefill.Account
		opts.Provider = subOpts.Provider
		opts.Email = subOpts.Email
		opts.Account = subOpts.Account
		return nil
	}
	if needsMailFields(stepID) && strings.TrimSpace(subOpts.Email) != "" {
		writeTTYLine(out, clui.Muted("Using saved mailbox "+strings.TrimSpace(subOpts.Email)))
	}
	return nil
}
