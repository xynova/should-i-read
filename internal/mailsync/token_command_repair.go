package mailsync

import (
	"os"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// EnsureMailSyncTokenCommand rewrites mail-sync.toml when token.command omits --account
// for a labeled account (Neverest must match keyring namespace used at login).
func EnsureMailSyncTokenCommand(path, hostBin string) error {
	const op = "mailsync.EnsureMailSyncTokenCommand"
	path = strings.TrimSpace(path)
	if path == "" {
		return sirerr.New(sirerr.CodeInvalid, op, "mail sync config path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "read mail sync config").With("path", path)
	}
	body := string(raw)
	account := AccountFromMailSyncTOML(body)
	if account == "" {
		return nil
	}
	provider := InferProviderFromMailSyncTOML(body)
	if provider == "" {
		return nil
	}
	wantTok, err := formatTokenCommandTOML(buildTokenArgv(hostBin, provider, account))
	if err != nil {
		return err
	}
	if strings.Contains(body, wantTok) {
		return nil
	}
	email := EmailFromMailSyncTOML(body)
	storeRoot := StoreRootFromMailSyncTOML(body)
	if email == "" || storeRoot == "" {
		return sirerr.New(sirerr.CodeFailed, op, "mail sync config missing email or store.root").
			With("path", path)
	}
	newBody, err := renderAccountTOML(accountTemplateData{
		Account:          account,
		IMAPServer:       provider.imapServer(),
		Email:            email,
		StoreRoot:        storeRoot,
		TokenCommandTOML: wantTok,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(newBody), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write mail sync config").With("path", path)
	}
	return nil
}
