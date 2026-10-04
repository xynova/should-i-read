package configure

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// HuhHubPrompter is the production TTY hub prompter.
type HuhHubPrompter struct{}

func (HuhHubPrompter) ChooseAction(ctx context.Context, snap Snapshot) (HubAction, error) {
	const op = "configure.HuhHubPrompter.ChooseAction"
	if err := ctx.Err(); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	var choice string
	opts := []huh.Option[string]{
		huh.NewOption("Fix a step…", string(HubFixStep)),
		huh.NewOption("Quit", string(HubQuit)),
	}
	if !snap.Ready {
		opts = append([]huh.Option[string]{
			huh.NewOption("Run all missing steps", string(HubRunMissing)),
		}, opts...)
	} else {
		opts = []huh.Option[string]{
			huh.NewOption("Done", string(HubQuit)),
			huh.NewOption("Re-run a step…", string(HubFixStep)),
		}
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What next?").
				Options(opts...).
				Value(&choice),
		),
	).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		return "", mapHuhErr(op, err)
	}
	return HubAction(choice), nil
}

func (HuhHubPrompter) ChooseStep(ctx context.Context, snap Snapshot) (string, error) {
	const op = "configure.HuhHubPrompter.ChooseStep"
	if err := ctx.Err(); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	var choice string
	opts := make([]huh.Option[string], 0, len(snap.Steps))
	// Prefer incomplete steps first so the cursor lands on work to do.
	for _, preferOK := range []bool{false, true} {
		for _, s := range snap.Steps {
			if s.OK != preferOK {
				continue
			}
			label := fmt.Sprintf("%s  %s", markGlyph(s.OK), StepTitle(s.ID))
			if d := strings.TrimSpace(s.Detail); d != "" {
				label += "  ·  " + shorten(d, 40)
			}
			opts = append(opts, huh.NewOption(label, s.ID))
		}
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which step?").
				Options(opts...).
				Value(&choice),
		),
	).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		return "", mapHuhErr(op, err)
	}
	return choice, nil
}

func markGlyph(ok bool) string {
	if ok {
		return "✓"
	}
	return "✗"
}

func (HuhHubPrompter) ConfirmBrowserLogin(ctx context.Context) (bool, error) {
	const op = "configure.HuhHubPrompter.ConfirmBrowserLogin"
	if err := ctx.Err(); err != nil {
		return false, sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	var ok bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Open the browser for mailbox login now?").
				Affirmative("Yes").
				Negative("Not now").
				Value(&ok),
		),
	).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		return false, mapHuhErr(op, err)
	}
	return ok, nil
}

func (HuhHubPrompter) ConfirmForceOverwrite(ctx context.Context, path string) (bool, error) {
	const op = "configure.HuhHubPrompter.ConfirmForceOverwrite"
	_ = ctx
	_ = path
	var ok bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Overwrite existing mailbox settings?").
				Affirmative("Overwrite").
				Negative("Cancel").
				Value(&ok),
		),
	).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		return false, mapHuhErr(op, err)
	}
	return ok, nil
}

func (HuhHubPrompter) CollectMailboxFields(ctx context.Context, prefill MailboxPrefill) (MailboxPrefill, error) {
	const op = "configure.HuhHubPrompter.CollectMailboxFields"
	if err := ctx.Err(); err != nil {
		return prefill, sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	provider := strings.TrimSpace(prefill.Provider)
	email := strings.TrimSpace(prefill.Email)
	account := strings.TrimSpace(prefill.Account)
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Mail provider").
				Options(
					huh.NewOption("Gmail", "gmail"),
					huh.NewOption("Outlook / Microsoft 365", "outlook"),
				).
				Value(&provider),
			huh.NewInput().
				Title("Mailbox email address").
				Value(&email).
				Validate(validateMailboxEmail),
			huh.NewInput().
				Title("Account label (optional)").
				Placeholder("gmail").
				Value(&account),
		),
	).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		return prefill, mapHuhErr(op, err)
	}
	return MailboxPrefill{
		Provider: strings.TrimSpace(provider),
		Email:    strings.TrimSpace(email),
		Account:  strings.TrimSpace(account),
	}, nil
}

func validateMailboxEmail(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || !strings.Contains(s, "@") {
		return errors.New("enter a valid mailbox address")
	}
	return nil
}

func mapHuhErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return sirerr.Wrap(err, sirerr.CodeInvalid, op, "configure canceled")
	}
	return sirerr.Wrap(err, sirerr.CodeFailed, op, "prompt failed")
}
