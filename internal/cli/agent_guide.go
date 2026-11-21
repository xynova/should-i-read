package cli

import (
	"fmt"
	"io"
	"strings"
)

func writeln(w io.Writer, format string, args ...any) {
	if format == "" {
		fmt.Fprintln(w)
		return
	}
	out := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	fmt.Fprint(w, out)
}

func agentOperatingGuide(w io.Writer) {
	writeln(w, "should-i-read %s", Version)
	writeln(w, "Local inbox triage: mail sync, Polypus, report-only.")
	writeln(w, "")
	writeln(w, "Bare invoke does not start overview serve or mutate mail.")
	writeln(w, "")
	writeln(w, "ROLE")
	writeln(w, "  Operator CLI for mail sync, classify report, and confirmed cart apply.")
	writeln(w, "")
	writeln(w, "AGENT OPERATING GUIDE")
	writeln(w, "  AGENTS.md")
	writeln(w, "  ai-copilots/skills/should-i-read-operator/SKILL.md")
	writeln(w, "")
	writeln(w, "COMMANDS BY RISK")
	writeln(w, "  Read-only")
	writeln(w, "    version, help, polypus check, mail status, mail readiness, mail overview")
	writeln(w, "  Changes host or remote")
	writeln(w, "    init, configure, mail sync, mail cart apply, mail overview serve")
	writeln(w, "")
	writeln(w, "AUTOMATION RULES")
	writeln(w, "  Fail closed on Polypus health before AI work.")
	writeln(w, "  Classify and report stay report-only until eval exit criteria pass.")
	writeln(w, "  Trash apply needs confirm token trash-at-source.")
	writeln(w, "  Example: should-i-read configure --apply --provider gmail --email you@example.com")
	writeln(w, "")
	writeln(w, "Full command list: should-i-read help")
}
