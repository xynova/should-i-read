package mailsync

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/template"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const accountTOML = `# Mail sync account written by should-i-read configure.
# OAuth tokens are not stored here; token.command calls the host CLI.

[accounts.{{.Account}}]
default = true
retain = true
imap.server = "{{.IMAPServer}}"
imap.collection.filter = "all"
imap.sasl.xoauth2.username = "{{.Email}}"
imap.sasl.xoauth2.token.command = {{.TokenCommandTOML}}
store.root = "{{.StoreRoot}}"
`

type accountTemplateData struct {
	Account          string
	IMAPServer       string
	Email            string
	StoreRoot        string
	TokenCommandTOML string
}

func renderAccountTOML(data accountTemplateData) (string, error) {
	const op = "mailsync.renderAccountTOML"
	t, err := template.New("account").Parse(accountTOML)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "parse template")
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "execute template")
	}
	return buf.String(), nil
}

func formatTokenCommandTOML(argv []string) (string, error) {
	const op = "mailsync.formatTokenCommandTOML"
	if len(argv) == 0 {
		return "", sirerr.New(sirerr.CodeInvalid, op, "token command argv is empty")
	}
	raw, err := json.Marshal(argv)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "marshal argv")
	}
	return string(raw), nil
}

func buildTokenArgv(hostBin string, provider Provider, accountLabel string) []string {
	bin := strings.TrimSpace(hostBin)
	if bin == "" {
		bin = "should-i-read"
	}
	argv := []string{bin, "token", provider.tokenSubcommand()}
	if label := strings.TrimSpace(accountLabel); label != "" {
		argv = append(argv, "--account", label)
	}
	return argv
}
