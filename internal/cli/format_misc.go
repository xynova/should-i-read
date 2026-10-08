package cli

import (
	"fmt"
	"strings"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/configure"
	"github.com/xynova/should-i-read/internal/mailsync"
	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/setup"
)

func formatPolypusHealth(h *polypus.HealthResult, classifyChecked bool) string {
	if h == nil {
		return clui.FormatBox("Polypus", clui.Miss("No result"))
	}
	var b strings.Builder
	b.WriteString(clui.OK("Polypus OK"))
	b.WriteByte('\n')
	b.WriteString(clui.Muted(h.BaseURL))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf("%d model(s) enabled", h.ModelCount))
	if len(h.ModelIDs) > 0 {
		b.WriteByte('\n')
		ids := h.ModelIDs
		if len(ids) > 8 {
			ids = ids[:8]
		}
		b.WriteString(clui.Muted(strings.Join(ids, ", ")))
		if len(h.ModelIDs) > 8 {
			b.WriteString(clui.Muted(" …"))
		}
	}
	b.WriteByte('\n')
	if classifyChecked && h.ClassifyReady {
		b.WriteString(clui.OK("Classify ready"))
		b.WriteByte('\n')
		b.WriteString(clui.Muted(fmt.Sprintf("judge %s (%s)", h.JudgeModel, h.JudgeSource)))
		b.WriteByte('\n')
		if h.AuthorSource != "" {
			b.WriteString(clui.Muted(fmt.Sprintf("author %s (%s)", h.AuthorModel, h.AuthorSource)))
		} else {
			b.WriteString(clui.Muted(fmt.Sprintf("author %s", h.AuthorModel)))
		}
	} else if !classifyChecked {
		b.WriteString(clui.Muted("Classify: not verified (use --classify)"))
	}
	return clui.FormatBox("Polypus", strings.TrimRight(b.String(), "\n"))
}

func formatConfigRedacted(m map[string]any) string {
	var b strings.Builder
	if p, ok := m["path"].(string); ok && p != "" {
		b.WriteString("config: ")
		b.WriteString(p)
		b.WriteByte('\n')
	}
	if pol, ok := m["polypus"].(map[string]string); ok {
		b.WriteString("polypus.base_url: ")
		b.WriteString(pol["base_url"])
		b.WriteByte('\n')
	}
	if pim, ok := m["pimalaya"].(map[string]string); ok {
		for _, k := range []string{"neverest_config", "default_account", "pimdir_path"} {
			if v := strings.TrimSpace(pim[k]); v != "" {
				b.WriteString("pimalaya." + k + ": ")
				b.WriteString(v)
				b.WriteByte('\n')
			}
		}
	}
	if oauth, ok := m["oauth"].(map[string]string); ok {
		b.WriteString("oauth.gmail_client_id: ")
		b.WriteString(oauth["gmail_client_id"])
		b.WriteByte('\n')
		b.WriteString("oauth.gmail_client_secret: ")
		b.WriteString(oauth["gmail_client_secret"])
		b.WriteByte('\n')
		b.WriteString("oauth.outlook_client_id: ")
		b.WriteString(oauth["outlook_client_id"])
		b.WriteByte('\n')
	}
	return clui.FormatBox("Config", strings.TrimRight(b.String(), "\n"))
}

func formatConfigBump(path string, changed bool, notes []string) string {
	var b strings.Builder
	b.WriteString(clui.Muted(path))
	b.WriteByte('\n')
	if changed {
		b.WriteString(clui.OK("Updated"))
	} else {
		b.WriteString(clui.Muted("No changes needed"))
	}
	for _, n := range notes {
		b.WriteByte('\n')
		b.WriteString(clui.Muted("  · " + n))
	}
	return clui.FormatBox("Config bump", strings.TrimRight(b.String(), "\n"))
}

func formatConfigureSession(session configure.SessionResult) string {
	var b strings.Builder
	b.WriteString(configure.FormatChecklist(session.Snapshot))
	b.WriteByte('\n')
	if len(session.Ran) > 0 {
		b.WriteString(clui.Title("Ran"))
		b.WriteByte('\n')
		for _, r := range session.Ran {
			mark := clui.MarkMiss()
			if r.OK {
				mark = clui.MarkOK()
			}
			b.WriteString(fmt.Sprintf("%s  %s\n", mark, configure.StepTitle(r.Step)))
		}
	}
	b.WriteString(configure.FormatHubSummary(session.Snapshot))
	return strings.TrimRight(b.String(), "\n")
}

func formatSetupResult(res setup.Result) string {
	var b strings.Builder
	b.WriteString(clui.Muted("config: " + res.ConfigPath))
	b.WriteByte('\n')
	b.WriteString(clui.Muted("mode: " + string(res.Mode)))
	b.WriteByte('\n')
	for _, step := range res.NextSteps {
		b.WriteString(clui.Hint(step))
		b.WriteByte('\n')
	}
	return clui.FormatBox("OAuth setup", strings.TrimRight(b.String(), "\n"))
}

func mailSetupStepLabel(id string) string {
	switch strings.TrimSpace(id) {
	case "ensure":
		return "Mail sync dependency"
	case "configure":
		return "Mailbox config"
	case "login":
		return "Mailbox login"
	case "init":
		return "Local mail store init"
	default:
		if id == "" {
			return "step"
		}
		return id
	}
}

func formatMailSetupResult(res mailsync.SetupResult) string {
	var b strings.Builder
	for _, step := range res.Steps {
		mark := clui.MarkMiss()
		if step.OK {
			mark = clui.MarkOK()
		} else if step.Skipped {
			mark = clui.Muted("–")
		}
		line := mailSetupStepLabel(step.ID)
		if d := strings.TrimSpace(step.Detail); d != "" {
			line += "  " + clui.Muted(d)
		}
		b.WriteString(fmt.Sprintf("%s  %s\n", mark, line))
	}
	if w := strings.TrimSpace(res.WaitingOn); w != "" {
		b.WriteString("\n")
		b.WriteString(clui.Hint(w))
	}
	title := "Mail setup"
	if res.Status.Ready {
		title = "Mail setup OK"
	}
	return clui.FormatBox(title, strings.TrimRight(b.String(), "\n"))
}

func formatMailConfigureResult(res mailsync.Result) string {
	var b strings.Builder
	if res.Email != "" {
		b.WriteString(clui.Muted(res.Email))
		b.WriteByte('\n')
	}
	if res.Provider != "" {
		b.WriteString(fmt.Sprintf("%s / %s\n", res.Provider, res.Account))
	}
	if p := strings.TrimSpace(res.MailSyncConfig); p != "" {
		b.WriteString(clui.Muted("mail-sync: " + p))
		b.WriteByte('\n')
	}
	if p := strings.TrimSpace(res.PimdirPath); p != "" {
		b.WriteString(clui.Muted("pimdir: " + p))
		b.WriteByte('\n')
	}
	for _, step := range res.NextSteps {
		b.WriteString(clui.Hint(step))
		b.WriteByte('\n')
	}
	return clui.FormatBox("Mail configure", strings.TrimRight(b.String(), "\n"))
}

func formatTokenStatus(m map[string]string) string {
	var b strings.Builder
	for k, v := range m {
		mark := clui.MarkMiss()
		if v == "(set)" {
			mark = clui.MarkOK()
		}
		b.WriteString(fmt.Sprintf("%s  %s: %s\n", mark, k, v))
	}
	return clui.FormatBox("Token status", strings.TrimRight(b.String(), "\n"))
}
