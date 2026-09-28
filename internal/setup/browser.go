package setup

import (
	"context"
	"os/exec"
	"runtime"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// systemBrowser opens URLs with the OS default handler.
type systemBrowser struct{}

func (systemBrowser) Open(ctx context.Context, url string) error {
	const op = "setup.browser.Open"
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "context is nil")
	}
	if err := ctx.Err(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "context canceled")
	}
	if strings.TrimSpace(url) == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "url is empty")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "start browser").With("url", url)
	}
	_ = cmd.Process.Release()
	return nil
}

func openGuidedDocs(ctx context.Context, b Browser, providers Provider) error {
	const op = "setup.openGuidedDocs"
	if b == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "browser is nil")
	}
	var first error
	open := func(url string) {
		if err := b.Open(ctx, url); err != nil && first == nil {
			first = err
		}
	}
	switch providers {
	case ProviderGmail:
		open(GmailAPIURL)
		open(GmailConsoleURL)
	case ProviderOutlook:
		open(OutlookPortalURL)
	case ProviderBoth:
		open(GmailAPIURL)
		open(GmailConsoleURL)
		open(OutlookPortalURL)
	default:
		return sirerr.New(sirerr.CodeInvalid, op, "unknown providers").With("providers", string(providers))
	}
	if first != nil {
		return sirerr.Wrap(first, sirerr.CodeFailed, op, "open documentation")
	}
	return nil
}
