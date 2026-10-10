package mailsync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestSetupMissingEmail(t *testing.T) {
	dir := t.TempDir()
	initHostConfig(t, dir)
	w, err := CreateWorkflow(dir, "/bin/fake", WorkflowHooks{
		EnsureDep: func(ctx context.Context, installIfMissing bool) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Setup(context.Background(), SetupOptions{Provider: ProviderGmail})
	if err == nil {
		t.Fatal("expected error")
	}
	if code, ok := sirerr.AsCode(err); !ok || code != sirerr.CodeInvalid {
		t.Fatalf("got %v", err)
	}
}

func TestSetupWaitingOnLoginNonTTY(t *testing.T) {
	dir := t.TempDir()
	initHostConfig(t, dir)
	var initCalled bool
	noToken := func(Provider, string) bool { return false }
	w, err := CreateWorkflow(dir, "/bin/fake", WorkflowHooks{
		EnsureDep:  func(ctx context.Context, installIfMissing bool) error { return nil },
		TokenReady: noToken,
		InitReplica: func(ctx context.Context, cfg config.Config, account string) error {
			initCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Setup(context.Background(), SetupOptions{
		Provider:    ProviderGmail,
		Email:       "me@example.com",
		Interactive: false,
	})
	if err != nil {
		t.Fatalf("expected success waiting: %v", err)
	}
	if res.WaitingOn != waitingOnMailboxLogin {
		t.Fatalf("waiting_on=%q", res.WaitingOn)
	}
	if initCalled {
		t.Fatal("init should not run without token")
	}
	if len(res.Steps) < 3 {
		t.Fatalf("steps=%v", res.Steps)
	}
}

func TestSetupSkipLogin(t *testing.T) {
	dir := t.TempDir()
	initHostConfig(t, dir)
	w, err := CreateWorkflow(dir, "/bin/fake", WorkflowHooks{
		EnsureDep:  func(ctx context.Context, installIfMissing bool) error { return nil },
		TokenReady: func(Provider, string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Setup(context.Background(), SetupOptions{
		Provider:    ProviderGmail,
		Email:       "a@b.com",
		SkipLogin:   true,
		Interactive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.WaitingOn != waitingOnMailboxLogin {
		t.Fatalf("waiting_on=%q", res.WaitingOn)
	}
}

func TestSetupEnsureFailure(t *testing.T) {
	dir := t.TempDir()
	initHostConfig(t, dir)
	w, err := CreateWorkflow(dir, "/bin/fake", WorkflowHooks{
		EnsureDep: func(ctx context.Context, installIfMissing bool) error {
			return sirerr.New(sirerr.CodeUnavailable, "test", "fail")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Setup(context.Background(), SetupOptions{
		Provider: ProviderGmail,
		Email:    "a@b.com",
	})
	if code, ok := sirerr.AsCode(err); err == nil || !ok || code != sirerr.CodeUnavailable {
		t.Fatalf("got %v", err)
	}
}

func initHostConfig(t *testing.T, repoRoot string) {
	t.Helper()
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
	_ = repoRoot
}
