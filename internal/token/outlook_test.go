package token

import (
	"context"
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/config"
)

func TestOutlookMSALCacheKey(t *testing.T) {
	t.Parallel()
	if OutlookMSALCacheKey("") != KeyOutlookMSALCache {
		t.Fatal()
	}
	if OutlookMSALCacheKey("work@contoso.com") != "OUTLOOK_MSAL_CACHE__work_contoso.com" {
		t.Fatalf("%q", OutlookMSALCacheKey("work@contoso.com"))
	}
}

func TestSaveLoadOutlookMSALCacheRoundTrip(t *testing.T) {
	withMemKeyring(t)
	payload := []byte{0x01, 0x02, 0xff, 0x00, 0xab}
	if err := SaveOutlookMSALCache("msal-roundtrip", payload); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOutlookMSALCache("msal-roundtrip")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %v want %v", got, payload)
	}
}

func TestOutlookAccessTokenMissingCache(t *testing.T) {
	withMemKeyring(t)
	t.Setenv(KeyOutlookMSALCache, "")
	cfg := config.Config{OutlookClientID: "00000000-0000-0000-0000-000000000001"}
	_, err := outlookAccessToken(context.Background(), cfg, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "login") {
		t.Fatalf("%v", err)
	}
}
