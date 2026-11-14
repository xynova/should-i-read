package mailsync

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const himalayaPimdirTOML = `# Himalaya pimdir account written by should-i-read configure.
# Offline replica only; Neverest mail sync pushes staged edits.

[accounts.{{.Account}}]
default = true
pimdir.root = {{.PimdirRootQuoted}}
pimdir.account = "{{.Account}}"
mailbox.alias.trash = "{{.Account}}/imap/{{.TrashAlias}}"
`

type himalayaTemplateData struct {
	Account          string
	PimdirRootQuoted string
	TrashAlias       string
}

func renderHimalayaTOML(account, pimdirRoot, trashAlias string) (string, error) {
	const op = "mailsync.renderHimalayaTOML"
	account = strings.TrimSpace(account)
	pimdirRoot = strings.TrimSpace(pimdirRoot)
	trashAlias = strings.TrimSpace(trashAlias)
	if account == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "account is required")
	}
	if pimdirRoot == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "pimdir root is required")
	}
	if trashAlias == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "trash mailbox alias is required")
	}
	t, err := template.New("himalaya").Parse(himalayaPimdirTOML)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "parse template")
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, himalayaTemplateData{
		Account:          account,
		PimdirRootQuoted: fmt.Sprintf("%q", pimdirRoot),
		TrashAlias:       trashAlias,
	}); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "execute template")
	}
	return buf.String(), nil
}
