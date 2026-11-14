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
	if loaded.Pimalaya.HimalayaConfig != res.HimalayaConfig {
		t.Fatalf("himalaya_config: %q", loaded.Pimalaya.HimalayaConfig)
	}
	if _, err := os.Stat(res.HimalayaConfig); err != nil {
		t.Fatalf("himalaya.toml: %v", err)
	}
	if !res.HimalayaWritten {
		t.Fatal("expected himalaya written")
	}
}

func TestConfigureWritesHimalayaWhenMailSyncAlreadyMatches(t *testing.T) {
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

	svc, err := Create(dir, "/bin/should-i-read", func(context.Context, config.Config, string) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{Provider: ProviderGmail, Email: "me@example.com", HostBin: "/bin/should-i-read", SkipInit: true}
	res1, err := svc.Configure(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	himalayaPath := res1.HimalayaConfig
	if err := os.Remove(himalayaPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	res2, err := svc.Configure(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if res2.ConfigWritten {
		t.Fatal("mail sync should not rewrite")
	}
	if !res2.AlreadyConfigured {
		t.Fatal("expected already configured mail sync")
	}
	if !res2.HimalayaWritten {
		t.Fatal("expected himalaya written")
	}
	if _, err := os.Stat(himalayaPath); err != nil {
		t.Fatalf("himalaya.toml: %v", err)
	}
}

func TestConfigureRefusesDifferentHimalayaWithoutForce(t *testing.T) {
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
	opts := Options{Provider: ProviderGmail, Email: "me@example.com", HostBin: "/bin/x", SkipInit: true}
	res, err := svc.Configure(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(res.HimalayaConfig, []byte("# stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Configure(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error")
	}
	got, rerr := os.ReadFile(res.HimalayaConfig)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(got) != "# stale\n" {
		t.Fatalf("himalaya.toml changed: %q", got)
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
