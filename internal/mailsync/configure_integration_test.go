package mailsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
	"golang.org/x/oauth2"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/token"
)

func TestConfigureWritesTomlAndPatchesYAML(t *testing.T) {
	dir := t.TempDir()
	xdg := filepath.Join(dir, "xdg")
	appCfgDir := filepath.Join(xdg, config.AppName)
	if err := os.MkdirAll(appCfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(appCfgDir, "config.yaml")
	example, err := config.ExampleYAML()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, example, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)

	mem := operatorconfig.NewMemKeyring()
	prev := operatorconfig.DefaultKeyring()
	operatorconfig.SetTestKeyring(mem)
	t.Cleanup(func() { operatorconfig.SetTestKeyring(prev) })
	if err := token.SaveGmailToken("gmail", &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	initCalled := false
	svc, err := Create(dir, "/bin/should-i-read", func(ctx context.Context, cfg config.Config, account string) error {
		initCalled = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.Configure(context.Background(), Options{
		Provider: ProviderGmail,
		Email:    "me@example.com",
		HostBin:  "/bin/should-i-read",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.ConfigWritten {
		t.Fatal("expected config written")
	}
	if !initCalled {
		t.Fatal("expected init")
	}
	if _, err := os.Stat(res.MailSyncConfig); err != nil {
		t.Fatalf("mail sync config: %v", err)
	}
	loaded, err := config.Load(dir, cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Pimalaya.NeverestConfig != res.MailSyncConfig {
		t.Fatalf("neverest_config: %q", loaded.Pimalaya.NeverestConfig)
	}
	if loaded.Pimalaya.PimdirPath != res.PimdirPath {
		t.Fatalf("pimdir_path: %q", loaded.Pimalaya.PimdirPath)
	}
}

func TestConfigureRefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	xdg := filepath.Join(dir, "xdg")
	appCfgDir := filepath.Join(xdg, config.AppName)
	if err := os.MkdirAll(appCfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(appCfgDir, "config.yaml")
	example, err := config.ExampleYAML()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, example, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", xdg)

	svc, err := Create(dir, "/bin/should-i-read", func(context.Context, config.Config, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Provider: ProviderGmail, Email: "a@example.com", HostBin: "/bin/x"}
	if _, err := svc.Configure(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	opts2 := Options{Provider: ProviderGmail, Email: "b@example.com", HostBin: "/bin/x"}
	_, err = svc.Configure(context.Background(), opts2)
	if err == nil {
		t.Fatal("expected error when mail sync config exists with different content")
	}
}
