package setup

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// HuhPrompter is the interactive TUI backed by charmbracelet/huh.
type HuhPrompter struct{}

// SelectMode asks how to supply OAuth client credentials.
func (HuhPrompter) SelectMode(ctx context.Context, productReady bool) (Mode, error) {
	const op = "setup.HuhPrompter.SelectMode"
	if err := ctx.Err(); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	_ = productReady
	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("How do you want to supply mail OAuth client credentials?").
				Description("Product-owned embeds are preferred when present. BYO is for org or sovereignty overrides. Guided DIY opens Cloud/Azure consoles.").
				Options(
					huh.NewOption("Paste BYO / org-provided client credentials", string(ModeBYO)),
					huh.NewOption("Guided DIY (Cloud Console / Entra) — advanced", string(ModeGuided)),
					huh.NewOption("Try gcloud DIY (advanced; fail closed to guided)", string(ModeGCloud)),
				).
				Value(&choice),
		),
	)
	if err := form.Run(); err != nil {
		return "", mapHuhErr(op, err)
	}
	return Mode(choice), nil
}

// SelectProviders asks which providers to configure.
func (HuhPrompter) SelectProviders(ctx context.Context) (Provider, error) {
	const op = "setup.HuhPrompter.SelectProviders"
	if err := ctx.Err(); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which mail providers?").
				Options(
					huh.NewOption("Gmail only", string(ProviderGmail)),
					huh.NewOption("Outlook only", string(ProviderOutlook)),
					huh.NewOption("Both Gmail and Outlook", string(ProviderBoth)),
				).
				Value(&choice),
		),
	)
	if err := form.Run(); err != nil {
		return "", mapHuhErr(op, err)
	}
	return Provider(choice), nil
}

// PromptGmail collects Gmail client id and optional Desktop secret (masked).
func (HuhPrompter) PromptGmail(ctx context.Context) (clientID, clientSecret string, err error) {
	const op = "setup.HuhPrompter.PromptGmail"
	if err := ctx.Err(); err != nil {
		return "", "", sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Gmail OAuth client id").
				Value(&clientID).
				Validate(func(s string) error {
					return ValidateGmailClientID(s)
				}),
			huh.NewInput().
				Title("Gmail Desktop client secret (optional; leave blank if unused)").
				EchoMode(huh.EchoModePassword).
				Value(&clientSecret),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", mapHuhErr(op, err)
	}
	return strings.TrimSpace(clientID), strings.TrimSpace(clientSecret), nil
}

// PromptOutlook collects the Entra public client id (no secret).
func (HuhPrompter) PromptOutlook(ctx context.Context) (clientID string, err error) {
	const op = "setup.HuhPrompter.PromptOutlook"
	if err := ctx.Err(); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Outlook / Entra application (client) id").
				Description("Public client only; do not paste a client secret.").
				Value(&clientID).
				Validate(func(s string) error {
					return ValidateOutlookClientID(s)
				}),
		),
	)
	if err := form.Run(); err != nil {
		return "", mapHuhErr(op, err)
	}
	return strings.TrimSpace(clientID), nil
}

// Confirm asks a yes/no question.
func (HuhPrompter) Confirm(ctx context.Context, message string) (bool, error) {
	const op = "setup.HuhPrompter.Confirm"
	if err := ctx.Err(); err != nil {
		return false, sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	var ok bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(message).
				Value(&ok),
		),
	)
	if err := form.Run(); err != nil {
		return false, mapHuhErr(op, err)
	}
	return ok, nil
}

func mapHuhErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return sirerr.Wrap(err, sirerr.CodeInvalid, op, "setup canceled")
	}
	return sirerr.Wrap(err, sirerr.CodeFailed, op, "prompt failed")
}
