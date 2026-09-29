package token

import (
	"testing"
	"time"

	operatorconfig "github.com/behaviorengineering/operatorconfig/pkg/operatorconfig"
	"golang.org/x/oauth2"
)

func withMemKeyring(t *testing.T) {
	t.Setenv(KeyGmailOAuthToken, "")
	t.Setenv(EnvGmailRefreshToken, "")
	t.Setenv(KeyOutlookMSALCache, "")
	mem := operatorconfig.NewMemKeyring()
	prev := operatorconfig.DefaultKeyring()
	operatorconfig.SetTestKeyring(mem)
	t.Cleanup(func() { operatorconfig.SetTestKeyring(prev) })
}

func TestGmailTokenKey(t *testing.T) {
	t.Parallel()
	if GmailTokenKey("") != KeyGmailOAuthToken {
		t.Fatal()
	}
	if GmailTokenKey("work@gmail.com") != "GMAIL_OAUTH_TOKEN__work_gmail.com" {
		t.Fatalf("%q", GmailTokenKey("work@gmail.com"))
	}
}

func TestSaveLoadGmailTokenRoundTrip(t *testing.T) {
	withMemKeyring(t)
	tok := &oauth2.Token{
		AccessToken:  "access",
		TokenType:    "Bearer",
		RefreshToken: "refresh",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}
	if err := SaveGmailToken("roundtrip", tok); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGmailToken("roundtrip")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != tok.AccessToken || got.RefreshToken != tok.RefreshToken {
		t.Fatalf("got %+v", got)
	}
	if got.Expiry.Unix() != tok.Expiry.Unix() {
		t.Fatalf("expiry %v vs %v", got.Expiry, tok.Expiry)
	}
}

func TestLoadLegacyRefreshOnly(t *testing.T) {
	withMemKeyring(t)
	mem := operatorconfig.DefaultKeyring()
	if err := mem.Set("should-i-read", EnvGmailRefreshToken, "legacy-refresh"); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGmailToken("")
	if err != nil {
		t.Fatal(err)
	}
	if got.RefreshToken != "legacy-refresh" || got.AccessToken != "" {
		t.Fatalf("%+v", got)
	}
}

func TestTokenChanged(t *testing.T) {
	t.Parallel()
	a := &oauth2.Token{AccessToken: "a", Expiry: time.Now()}
	b := &oauth2.Token{AccessToken: "b", Expiry: time.Now()}
	if !TokenChanged(a, b) {
		t.Fatal("expected changed")
	}
	if TokenChanged(a, a) {
		t.Fatal("expected unchanged")
	}
}
