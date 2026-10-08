package configure

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/sirerr"
)

var stepTitles = map[string]string{
	StepHostConfig:     "Operator config",
	StepOAuthClients:   "OAuth app credentials",
	StepMailDependency: "Mail sync dependency",
	StepMailboxProfile: "Mailbox settings",
	StepMailboxLogin:   "Mailbox login",
	StepLocalStore:     "Local mail store",
}

// StepTitle returns a human label for a step id.
func StepTitle(id string) string {
	if t, ok := stepTitles[id]; ok {
		return t
	}
	return id
}

// ClearScreen clears the terminal when w is a TTY-like writer that supports ANSI.
func ClearScreen(w io.Writer) {
	if w == nil {
		return
	}
	fmt.Fprint(w, "\033[H\033[2J")
}

// FormatChecklist renders a compact readiness card (titles only; no repeated detail prose).
func FormatChecklist(snap Snapshot) string {
	var b strings.Builder
	b.WriteString(clui.Title("Configure"))
	b.WriteByte('\n')
	done, total := 0, len(snap.Steps)
	for _, s := range snap.Steps {
		if s.OK {
			done++
		}
	}
	progress := fmt.Sprintf("%d/%d ready", done, total)
	if email := strings.TrimSpace(snap.Mail.Email); email != "" {
		progress += "  ·  " + email
	}
	b.WriteString(clui.Muted(progress))
	b.WriteByte('\n')
	b.WriteByte('\n')
	for _, s := range snap.Steps {
		mark := clui.MarkMiss()
		if s.OK {
			mark = clui.MarkOK()
		}
		line := fmt.Sprintf("%s  %s", mark, StepTitle(s.ID))
		if !s.OK {
			if hint := checklistHint(s); hint != "" {
				line += clui.Muted("  ·  " + hint)
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if snap.Ready {
		b.WriteByte('\n')
		b.WriteString(clui.OK("All steps complete."))
		b.WriteByte('\n')
	}
	return clui.FormatBox("", strings.TrimRight(b.String(), "\n"))
}

func checklistHint(s StepStatus) string {
	switch s.ID {
	case StepHostConfig:
		return "missing"
	case StepOAuthClients:
		return "credentials missing"
	case StepMailDependency:
		return "not installed"
	case StepMailboxProfile:
		return "not saved"
	case StepMailboxLogin:
		return "not complete"
	case StepLocalStore:
		return "not initialized"
	default:
		return ""
	}
}

// FormatStepFailed prints a short operator-facing failure, preferring CLI stderr when present.
func FormatStepFailed(err error) string {
	if err == nil {
		return ""
	}
	msg, hint := humanFailure(err)
	out := clui.Err("Failed: ") + msg
	if hint != "" {
		out += "\n" + clui.Hint(hint)
	}
	return out
}

// FormatHubSummary is the quiet exit line after an interactive hub session.
func FormatHubSummary(snap Snapshot) string {
	if snap.Ready {
		return clui.OK("Ready.") + clui.Muted("  Next: make readiness && make sync")
	}
	waiting := snap.WaitingOn
	if waiting == "" {
		for _, s := range snap.Steps {
			if !s.OK {
				waiting = s.ID
				break
			}
		}
	}
	if waiting == "" {
		return clui.Miss("Not ready.") + "  Re-run: should-i-read configure"
	}
	return clui.Miss("Not ready.") + clui.Muted("  Next: fix "+StepTitle(waiting)+" (should-i-read configure)")
}

// applyMailFromSnapshot copies saved mailbox identity into empty option fields.
func applyMailFromSnapshot(opts *Options, snap Snapshot) {
	if opts == nil {
		return
	}
	if strings.TrimSpace(opts.Provider) == "" {
		opts.Provider = strings.TrimSpace(snap.Mail.Provider)
	}
	if strings.TrimSpace(opts.Email) == "" {
		opts.Email = strings.TrimSpace(snap.Mail.Email)
	}
	if strings.TrimSpace(opts.Account) == "" {
		opts.Account = strings.TrimSpace(snap.Mail.Account)
	}
}

// needsMailboxPrompt is true only when mail identity is still unknown after using the saved profile.
// See plan prompt matrix: ConfigOK profile is authoritative for login/store steps.
func needsMailboxPrompt(stepID string, opts Options, snap Snapshot) bool {
	if !needsMailFields(stepID) {
		return false
	}
	prov := strings.TrimSpace(opts.Provider)
	email := strings.TrimSpace(opts.Email)
	acct := strings.TrimSpace(opts.Account)
	if snap.Mail.ConfigOK {
		switch stepID {
		case StepMailboxProfile:
			return false
		case StepMailboxLogin, StepLocalStore:
			if prov != "" && email != "" {
				return false
			}
			if prov != "" && acct != "" && snap.Mail.TokenOK {
				return false
			}
		}
	}
	return prov == "" || email == ""
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

func humanFailure(err error) (msg, hint string) {
	if err == nil {
		return "", ""
	}
	hint = mailboxReuseHint(err)
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		var se *sirerr.Error
		if !errors.As(cur, &se) || se == nil {
			continue
		}
		if stderr := strings.TrimSpace(se.Fields["stderr"]); stderr != "" {
			return shorten(firstLine(stderr), 120), storeStepReuseHint()
		}
	}
	if msg := rootMessage(err); msg != "" {
		return msg, hint
	}
	return shorten(err.Error(), 100), hint
}

func storeStepReuseHint() string {
	return "Saved mailbox profile was reused; only the local store step failed."
}

func mailboxReuseHint(err error) string {
	s := err.Error()
	if strings.Contains(s, "Local mail store") || strings.Contains(s, "pimalaya") || strings.Contains(s, "neverest") {
		return "Mailbox login was already stored; re-run only initializes the local store."
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func rootMessage(err error) string {
	if err == nil {
		return ""
	}
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		var se *sirerr.Error
		if errors.As(cur, &se) && se != nil && se.Message != "" {
			if se.Message == "pimalaya CLI failed" {
				continue
			}
			if strings.HasPrefix(se.Message, "configure.") {
				continue
			}
			return shorten(se.Message, 100)
		}
	}
	s := err.Error()
	parts := strings.Split(s, ": ")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p == "" || p == "exit status 1" {
			continue
		}
		if strings.HasPrefix(p, "configure.") || strings.HasPrefix(p, "pimalaya.") {
			continue
		}
		if p == "pimalaya CLI failed" {
			continue
		}
		return shorten(p, 100)
	}
	return shorten(s, 100)
}
