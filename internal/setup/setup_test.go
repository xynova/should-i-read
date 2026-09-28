package setup

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/oauthcred"
	"github.com/xynova/should-i-read/internal/sirerr"
)

type fakePrompter struct {
	mode      Mode
	providers Provider
	gmailID   string
	gmailSec  string
	outlookID string
	confirm   bool
	modeErr   error
	gmailErr  error
}

func (f *fakePrompter) SelectMode(context.Context, bool) (Mode, error) {
	if f.modeErr != nil {
		return "", f.modeErr
	}
	return f.mode, nil
}

func (f *fakePrompter) SelectProviders(context.Context) (Provider, error) {
	return f.providers, nil
}

func (f *fakePrompter) PromptGmail(context.Context) (string, string, error) {
	if f.gmailErr != nil {
		return "", "", f.gmailErr
	}
	return f.gmailID, f.gmailSec, nil
}

func (f *fakePrompter) PromptOutlook(context.Context) (string, error) {
	return f.outlookID, nil
}

func (f *fakePrompter) Confirm(context.Context, string) (bool, error) {
	return f.confirm, nil
}

type fakeSecrets struct {
	vals map[string]string
	err  error
	fail string
}

func (f *fakeSecrets) Set(name, value string) error {
	if f.err != nil && (f.fail == "" || f.fail == name) {
		return f.err
	}
	if f.vals == nil {
		f.vals = make(map[string]string)
	}
	f.vals[name] = value
	return nil
}

type fakeBrowser struct {
	urls []string
	err  error
}

func (f *fakeBrowser) Open(_ context.Context, url string) error {
	f.urls = append(f.urls, url)
	return f.err
}

type fakeRunner struct {
	path    string
	pathErr error
	runErr  error
	calls   []string
}

func (f *fakeRunner) LookPath(string) (string, error) {
	if f.pathErr != nil {
		return "", f.pathErr
	}
	return f.path, nil
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if f.runErr != nil {
		return "", f.runErr
	}
	return "ok@example.com", nil
}

func TestValidateGmailClientID(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"ok", "123456789012-abcdef.apps.googleusercontent.com", false},
		{"empty", "", true},
		{"short", "abc", true},
		{"whitespace", "abc def ghi jkl", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateGmailClientID(tc.id)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateOutlookClientID(t *testing.T) {
	t.Parallel()
	if err := ValidateOutlookClientID("00000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOutlookClientID(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunProductReadySkipsPaste(t *testing.T) {
	var out bytes.Buffer
	wiz, err := Create(Config{
		RepoRoot:       t.TempDir(),
		Out:            &out,
		SkipInit:       true,
		ForceMode:      ModeProduct,
		HasProduct:     func() bool { return true },
		ResolveStatus:  func() map[string]string { return map[string]string{oauthcred.EnvGmailClientID: "(set)"} },
		ForceProviders: ProviderGmail,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := wiz.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Mode != ModeProduct {
		t.Fatalf("mode %q", res.Mode)
	}
}

func TestRunBYOStoresGmail(t *testing.T) {
	sec := &fakeSecrets{}
	prompter := &fakePrompter{
		mode:      ModeBYO,
		providers: ProviderGmail,
		gmailID:   "123456789012-abcdef.apps.googleusercontent.com",
		gmailSec:  "secret-value",
	}
	wiz, err := Create(Config{
		RepoRoot:       t.TempDir(),
		Out:            ioDiscard{},
		SkipInit:       true,
		ForceMode:      ModeBYO,
		ForceProviders: ProviderGmail,
		Prompter:       prompter,
		Secrets:        sec,
		HasProduct:     func() bool { return false },
		ResolveStatus: func() map[string]string {
			st := map[string]string{
				oauthcred.EnvGmailClientID:     "(unset)",
				oauthcred.EnvGmailClientSecret: "(unset)",
				oauthcred.EnvOutlookClientID:   "(unset)",
			}
			if _, ok := sec.vals[oauthcred.EnvGmailClientID]; ok {
				st[oauthcred.EnvGmailClientID] = "(set)"
			}
			if _, ok := sec.vals[oauthcred.EnvGmailClientSecret]; ok {
				st[oauthcred.EnvGmailClientSecret] = "(set)"
			}
			return st
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := wiz.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sec.vals[oauthcred.EnvGmailClientID] == "" {
		t.Fatal("expected gmail id stored")
	}
	if sec.vals[oauthcred.EnvGmailClientSecret] != "secret-value" {
		t.Fatal("expected gmail secret stored")
	}
	if res.Status[oauthcred.EnvGmailClientID] != "(set)" {
		t.Fatalf("status %+v", res.Status)
	}
	for _, n := range res.Notes {
		if strings.Contains(n, "secret-value") {
			t.Fatalf("note leaked secret: %q", n)
		}
	}
}

func TestPartialWriteErrorDoesNotLeakSecret(t *testing.T) {
	sec := secretFailNth{n: 2, fail: sirerr.New(sirerr.CodeFailed, "fake", "boom")}
	prompter := &fakePrompter{
		gmailID:  "123456789012-abcdef.apps.googleusercontent.com",
		gmailSec: "super-secret-do-not-leak",
	}
	wiz, err := Create(Config{
		RepoRoot:       t.TempDir(),
		Out:            ioDiscard{},
		SkipInit:       true,
		ForceMode:      ModeBYO,
		ForceProviders: ProviderGmail,
		Prompter:       prompter,
		Secrets:        &sec,
		HasProduct:     func() bool { return false },
		ResolveStatus:  func() map[string]string { return map[string]string{} },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = wiz.Run(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "super-secret-do-not-leak") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

type secretFailNth struct {
	n    int
	fail error
	i    int
	vals map[string]string
}

func (s *secretFailNth) Set(name, value string) error {
	s.i++
	if s.vals == nil {
		s.vals = make(map[string]string)
	}
	if s.i == s.n {
		return s.fail
	}
	s.vals[name] = value
	return nil
}

func TestGCloudMissingFailsClosedToGuided(t *testing.T) {
	browser := &fakeBrowser{}
	prompter := &fakePrompter{providers: ProviderGmail, confirm: false}
	runner := &fakeRunner{pathErr: errors.New("not found")}
	wiz, err := Create(Config{
		RepoRoot:       t.TempDir(),
		Out:            ioDiscard{},
		SkipInit:       true,
		ForceMode:      ModeGCloud,
		ForceProviders: ProviderGmail,
		Prompter:       prompter,
		Browser:        browser,
		Runner:         runner,
		HasProduct:     func() bool { return false },
		ResolveStatus:  func() map[string]string { return map[string]string{} },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := wiz.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Mode != ModeGCloud {
		t.Fatalf("mode %q", res.Mode)
	}
	found := false
	for _, n := range res.Notes {
		if strings.Contains(n, "gcloud not on PATH") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected fail-closed note, got %+v", res.Notes)
	}
	if len(browser.urls) == 0 {
		t.Fatal("expected guided docs opened")
	}
}

func TestGCloudRequiresDeadline(t *testing.T) {
	wiz, err := Create(Config{
		RepoRoot:       t.TempDir(),
		Out:            ioDiscard{},
		SkipInit:       true,
		ForceMode:      ModeGCloud,
		ForceProviders: ProviderGmail,
		Prompter:       &fakePrompter{confirm: false},
		Runner:         &fakeRunner{path: "/usr/bin/gcloud"},
		HasProduct:     func() bool { return false },
		ResolveStatus:  func() map[string]string { return map[string]string{} },
	})
	if err != nil {
		t.Fatal(err)
	}
	// Context without deadline must fail closed before process work.
	_, err = wiz.Run(context.Background())
	if err == nil {
		t.Fatal("expected deadline error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeInvalid {
		t.Fatalf("want invalid, got %v", err)
	}
}

func TestRunCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	wiz, err := Create(Config{
		RepoRoot:   t.TempDir(),
		Out:        ioDiscard{},
		SkipInit:   true,
		ForceMode:  ModeProduct,
		HasProduct: func() bool { return true },
		ResolveStatus: func() map[string]string {
			return map[string]string{}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = wiz.Run(ctx)
	if err == nil {
		t.Fatal("expected cancel error")
	}
}

func TestRunBYOProviderSelection(t *testing.T) {
	cases := []struct {
		name      string
		providers Provider
		wantKeys  []string
	}{
		{
			name:      "outlook",
			providers: ProviderOutlook,
			wantKeys:  []string{oauthcred.EnvOutlookClientID},
		},
		{
			name:      "both",
			providers: ProviderBoth,
			wantKeys: []string{
				oauthcred.EnvGmailClientID,
				oauthcred.EnvOutlookClientID,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sec := &fakeSecrets{}
			prompter := &fakePrompter{
				gmailID:   "123456789012-abcdef.apps.googleusercontent.com",
				outlookID: "00000000-0000-0000-0000-000000000001",
			}
			wiz, err := Create(Config{
				RepoRoot:       t.TempDir(),
				Out:            ioDiscard{},
				SkipInit:       true,
				ForceMode:      ModeBYO,
				ForceProviders: tc.providers,
				Prompter:       prompter,
				Secrets:        sec,
				HasProduct:     func() bool { return false },
				ResolveStatus:  func() map[string]string { return map[string]string{} },
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := wiz.Run(ctx); err != nil {
				t.Fatal(err)
			}
			for _, key := range tc.wantKeys {
				if sec.vals[key] == "" {
					t.Fatalf("missing stored key %s in %+v", key, sec.vals)
				}
			}
		})
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
