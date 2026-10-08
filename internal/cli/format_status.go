package cli

import (
	"fmt"
	"strings"

	"github.com/xynova/should-i-read/internal/clui"
	"github.com/xynova/should-i-read/internal/mailsync"
)

func formatMailStatus(st mailsync.Status) string {
	var b strings.Builder
	email := strings.TrimSpace(st.Email)
	if email != "" {
		b.WriteString(clui.Muted(email))
		b.WriteByte('\n')
	}
	lines := []struct {
		ok    bool
		label string
	}{
		{st.DepOK, "Mail sync dependency"},
		{st.ConfigOK, "Mailbox config"},
		{st.TokenOK, "Mailbox login"},
		{st.StoreOK, "Local mail store"},
	}
	for _, row := range lines {
		mark := clui.MarkMiss()
		if row.ok {
			mark = clui.MarkOK()
		}
		b.WriteString(fmt.Sprintf("%s  %s\n", mark, row.label))
	}
	if st.Ready {
		b.WriteString("\n")
		b.WriteString(clui.OK("Ready for make readiness && make sync"))
	} else if len(st.NextSteps) > 0 {
		b.WriteString("\n")
		b.WriteString(clui.Hint(st.NextSteps[0]))
	} else if len(st.Messages) > 0 {
		b.WriteString("\n")
		b.WriteString(clui.Hint(st.Messages[0]))
	}
	return clui.FormatBox("Mail status", strings.TrimRight(b.String(), "\n"))
}
