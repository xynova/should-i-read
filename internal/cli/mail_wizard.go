package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/xynova/should-i-read/internal/oauthcred"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// mailSetupUseWizard reports whether the interactive wizard should run.
func mailSetupUseWizard(tty, wizardFlag, providerFlagSet, emailFlagSet bool) bool {
	if !tty {
		return false
	}
	if wizardFlag {
		return true
	}
	return !providerFlagSet || !emailFlagSet
}

func runMailSetupWizard(ctx context.Context, flags mailSetupFlags) (mailSetupFlags, error) {
	const op = "cli.mail.setup.wizard"
	if ctx == nil {
		return flags, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	if err := ctx.Err(); err != nil {
		return flags, sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}

	out := flags
	provider := strings.TrimSpace(flags.provider)
	email := strings.TrimSpace(flags.email)
	account := strings.TrimSpace(flags.account)
	nextStep := VerbLogin
	if flags.skipLogin {
		nextStep = "save"
	}

	desc := "Installs the mail sync dependency, saves sync config, opens browser login, and initializes the local store."
	if !oauthcred.HasAny() {
		desc += " If login fails, run should-i-read configure and fix OAuth app credentials."
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Mailbox setup").
				Description(desc),
		),
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
				Placeholder("you@example.com").
				Value(&email).
				Validate(validateMailboxEmail),
			huh.NewInput().
				Title("Account label (optional)").
				Description("Use one label per inbox; defaults to the provider name.").
				Placeholder("gmail").
				Value(&account),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Next step").
				Options(
					huh.NewOption("Login", VerbLogin),
					huh.NewOption("Save config only", "save"),
				).
				Value(&nextStep),
		),
	)

	if err := form.Run(); err != nil {
		return flags, mapMailHuhErr(op, err)
	}

	out.provider = strings.TrimSpace(provider)
	out.email = strings.TrimSpace(email)
	out.account = strings.TrimSpace(account)
	out.skipLogin = nextStep == "save"
	return out, nil
}

func validateMailboxEmail(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || !strings.Contains(s, "@") || strings.HasPrefix(s, "@") {
		return sirerr.New(sirerr.CodeInvalid, "cli.validateMailboxEmail", "enter a valid mailbox address")
	}
	return nil
}

func mapMailHuhErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return sirerr.Wrap(err, sirerr.CodeInvalid, op, "mailbox setup canceled")
	}
	return sirerr.Wrap(err, sirerr.CodeFailed, op, "prompt failed")
}
