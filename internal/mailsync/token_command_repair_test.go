package mailsync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureMailSyncTokenCommand_addsAccountFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "mail-sync.toml")
	oldBody := `# test
[accounts.gmail]
default = true
retain = true
imap.server = "imap.gmail.com"
imap.collection.filter = "all"
imap.sasl.xoauth2.username = "me@example.com"
imap.sasl.xoauth2.token.command = ["/bin/sir","token","gmail"]
store.root = "/data/pim/gmail"
`
	if err := os.WriteFile(path, []byte(oldBody), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureMailSyncTokenCommand(path, "/bin/sir"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"--account","gmail"`) {
		t.Fatalf("expected --account in token command:\n%s", got)
	}
}
