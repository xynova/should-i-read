package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func formatPimalayaHuman(res *pimalaya.Result, accountHint string) string {
	if res == nil {
		return ""
	}
	rep := pimalaya.SummarizeNeverestJSON(res.Raw)
	if rep.Recognized && rep.Kind == "sync" {
		return formatSyncReport(rep)
	}
	if len(bytesTrim(res.Raw)) == 0 && res.Exit == 0 {
		acct := strings.TrimSpace(accountHint)
		if acct != "" {
			return clui.FormatBox("Readiness", clui.OK("Readiness OK")+"  "+clui.Muted("account "+acct))
		}
		return clui.FormatBox("Readiness", clui.OK("Readiness OK"))
	}
	if !rep.Recognized && len(bytesTrim(res.Raw)) > 0 {
		return clui.FormatBox("Mail engine", clui.Muted("Finished (unrecognized engine JSON). Use --json for the full payload."))
	}
	return clui.FormatBox("Mail engine", clui.OK("Finished"))
}

func formatPimalayaFailureHuman(res *pimalaya.Result, err error, accountHint string) string {
	var b strings.Builder
	title := "Sync failed"
	rep := pimalaya.SummarizeNeverestJSON(res.Raw)
	if rep.Recognized {
		code, ok := sirerr.AsCode(err)
		if rep.Conflicts > 0 || (ok && code == sirerr.CodeNeedsReview) {
			title = "Conflicts need review"
			b.WriteString(clui.Miss(fmt.Sprintf("%d outstanding conflict(s)", rep.Conflicts)))
			b.WriteByte('\n')
			b.WriteString(clui.Hint("Re-run with --json for the engine payload."))
			return clui.FormatBox(title, b.String())
		}
	}
	if rep.Recognized && rep.Kind == "sync" {
		b.WriteString(formatSyncReport(rep))
		b.WriteByte('\n')
	}
	msg := firstErrLine(err)
	if msg != "" {
		b.WriteString(clui.Err("Error: ") + msg)
		b.WriteByte('\n')
	}
	stderr := strings.TrimSpace(res.Stderr)
	if stderr != "" && !strings.Contains(msg, stderr) {
		b.WriteString(clui.Muted(shorten(stderr, 120)))
	}
	if b.Len() == 0 {
		b.WriteString(clui.Miss("Sync failed"))
	}
	return clui.FormatBox(title, strings.TrimRight(b.String(), "\n"))
}

func formatSyncReport(rep pimalaya.EngineReport) string {
	title := "Sync"
	if rep.DryRun {
		title = "Sync (dry-run)"
	}
	var b strings.Builder
	acct := strings.TrimSpace(rep.Account)
	if acct == "" {
		acct = "(default account)"
	}
	b.WriteString(clui.Muted("account "))
	b.WriteString(acct)
	b.WriteByte('\n')
	total := rep.Fetch + rep.Delete + rep.Other
	if total == 0 && rep.Conflicts == 0 {
		b.WriteString(clui.OK("Already up to date"))
		b.WriteByte('\n')
	} else {
		b.WriteString(fmt.Sprintf("Fetched %d · removed from local store %d", rep.Fetch, rep.Delete))
		if rep.Other > 0 {
			b.WriteString(fmt.Sprintf(" · other %d", rep.Other))
		}
		b.WriteByte('\n')
	}
	if len(rep.Folders) > 0 && len(rep.Folders) <= 8 {
		for _, f := range rep.Folders {
			b.WriteString(clui.Muted("  "))
			b.WriteString(f.Collection)
			b.WriteString(fmt.Sprintf(": +%d / -%d", f.Fetch, f.Delete))
			b.WriteByte('\n')
		}
	} else if len(rep.Folders) > 8 {
		b.WriteString(clui.Muted(fmt.Sprintf("  %d folders updated", len(rep.Folders))))
		b.WriteByte('\n')
	}
	if rep.HasGmailLabelOverlap() {
		b.WriteString(clui.Hint("Gmail labels overlap; counts are per folder, not unique messages."))
		b.WriteByte('\n')
	}
	if rep.Conflicts > 0 {
		b.WriteString(clui.Miss(fmt.Sprintf("Outstanding conflicts: %d", rep.Conflicts)))
		b.WriteByte('\n')
	} else if total > 0 {
		b.WriteString(clui.Hint("Next: make export"))
		b.WriteByte('\n')
	}
	return clui.FormatBox(title, strings.TrimRight(b.String(), "\n"))
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func firstErrLine(err error) string {
	if err == nil {
		return ""
	}
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		var se *sirerr.Error
		if errors.As(cur, &se) && se != nil && se.Message != "" {
			if se.Message == "pimalaya CLI failed" {
				continue
			}
			return shorten(se.Message, 100)
		}
	}
	return shorten(err.Error(), 100)
}

func shorten(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if max <= 0 || len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
