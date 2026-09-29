package token

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/xynova/should-i-read/internal/config"
)

func TestGmailAccessTokenRefreshPersists(t *testing.T) {
	withMemKeyring(t)
	const acct = "refresh-test"
	before := &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "refresh-1",
		Expiry:       time.Now().Add(-time.Hour),
	}
	after := &oauth2.Token{
		AccessToken:  "new-access",
		TokenType:    "Bearer",
		RefreshToken: "",
		Expiry:       time.Now().Add(time.Hour),
	}
	if err := SaveGmailToken(acct, before); err != nil {
		t.Fatal(err)
	}
	mergeRefreshToken(before, after)
	if err := SaveGmailToken(acct, after); err != nil {
		t.Fatal(err)
	}
	stored, err := LoadGmailToken(acct)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AccessToken != "new-access" || stored.RefreshToken != "refresh-1" {
		t.Fatalf("%+v", stored)
	}
}

func TestGmailAccessTokenMissingToken(t *testing.T) {
	withMemKeyring(t)
	t.Setenv(KeyGmailOAuthToken, "")
	t.Setenv(EnvGmailRefreshToken, "")
	cfg := config.Config{GmailClientID: "id", GmailClientSecret: "sec"}
	_, err := gmailAccessToken(context.Background(), cfg, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "login") {
		t.Fatalf("%v", err)
	}
}
