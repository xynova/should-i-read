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
{{.FilterLine}}
imap.sasl.xoauth2.username = "{{.Email}}"
imap.sasl.xoauth2.token.command = {{.TokenCommandTOML}}
store.root = "{{.StoreRoot}}"
`

type accountTemplateData struct {
	Account            string
	IMAPServer         string
	Email              string
	StoreRoot          string
	TokenCommandTOML   string
	FilterLine         string
	ExcludeCollections []string
}

func renderAccountTOML(data accountTemplateData) (string, error) {
	const op = "mailsync.renderAccountTOML"
	if strings.TrimSpace(data.FilterLine) == "" {
		line, err := collectionFilterLine(data.ExcludeCollections)
		if err != nil {
			return "", err
		}
		data.FilterLine = line
	}
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

func collectionFilterLine(excludes []string) (string, error) {
	const op = "mailsync.collectionFilterLine"
	names := uniqueTrimmedStrings(excludes)
	if len(names) == 0 {
		return `imap.collection.filter = "all"`, nil
	}
	raw, err := json.Marshal(names)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "marshal exclude list")
	}
	return "imap.collection.filter.exclude = " + string(raw), nil
}

func uniqueTrimmedStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func collectionFilterLineFromBody(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "imap.collection.filter") {
			return trim
		}
	}
	return `imap.collection.filter = "all"`
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
