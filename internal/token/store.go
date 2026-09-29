package token

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/xynova/should-i-read/internal/secret"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// KeyGmailOAuthToken is the keyring env name for the default Gmail account token JSON.
const KeyGmailOAuthToken = "GMAIL_OAUTH_TOKEN"

// EnvGmailRefreshToken is a legacy refresh-only secret (migrated on first refresh).
const EnvGmailRefreshToken = "GMAIL_OAUTH_REFRESH_TOKEN"

// KeyOutlookMSALCache is the keyring env name for the default Outlook MSAL cache blob (base64).
const KeyOutlookMSALCache = "OUTLOOK_MSAL_CACHE"

// tokenRecord is JSON stored in keyring (matches oauth2.Token fields).
type tokenRecord struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
}

// OutlookMSALCacheKey returns the keyring secret name for an Outlook account label (empty = default).
func OutlookMSALCacheKey(account string) string {
	account = sanitizeAccount(account)
	if account == "" {
		return KeyOutlookMSALCache
	}
	return KeyOutlookMSALCache + "__" + account
}

// GmailTokenKey returns the keyring secret name for an account label (empty = default).
func GmailTokenKey(account string) string {
	account = sanitizeAccount(account)
	if account == "" {
		return KeyGmailOAuthToken
	}
	return KeyGmailOAuthToken + "__" + account
}

func sanitizeAccount(account string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range account {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// LoadGmailToken reads oauth2.Token from keyring JSON or legacy refresh secret.
func LoadGmailToken(account string) (*oauth2.Token, error) {
	const op = "token.LoadGmailToken"
	key := GmailTokenKey(account)
	raw, err := secret.Resolve(key)
	if err == nil && strings.TrimSpace(raw) != "" {
		rec, err := decodeRecord(raw)
		if err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode gmail token json").With("key", key)
		}
		return recordToToken(rec), nil
	}
	legacyKey := legacyRefreshKey(account)
	refresh, err := secret.Resolve(legacyKey)
	if err != nil || strings.TrimSpace(refresh) == "" {
		return nil, sirerr.New(sirerr.CodeAuth, op, "gmail oauth token unset; run should-i-read token gmail login")
	}
	return &oauth2.Token{RefreshToken: refresh}, nil
}

func legacyRefreshKey(account string) string {
	if sanitizeAccount(account) == "" {
		return EnvGmailRefreshToken
	}
	return EnvGmailRefreshToken + "__" + sanitizeAccount(account)
}

// SaveGmailToken persists oauth2.Token JSON to keyring.
func SaveGmailToken(account string, tok *oauth2.Token) error {
	const op = "token.SaveGmailToken"
	if tok == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil token")
	}
	raw, err := encodeRecord(tok)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode gmail token json")
	}
	if err := secret.Set(GmailTokenKey(account), raw); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "keyring save gmail token")
	}
	return nil
}

// DeleteGmailToken removes stored Gmail oauth material for an account.
func DeleteGmailToken(account string) error {
	const op = "token.DeleteGmailToken"
	if err := secret.Delete(GmailTokenKey(account)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "delete gmail token json")
	}
	_ = secret.Delete(legacyRefreshKey(account))
	return nil
}

// GmailTokenStatus returns redacted storage status for an account.
func GmailTokenStatus(account string) string {
	key := GmailTokenKey(account)
	if v, err := secret.Resolve(key); err == nil && strings.TrimSpace(v) != "" {
		return "(set)"
	}
	if v, err := secret.Resolve(legacyRefreshKey(account)); err == nil && strings.TrimSpace(v) != "" {
		return "(legacy_refresh_only)"
	}
	return "(unset)"
}

func encodeRecord(tok *oauth2.Token) (string, error) {
	rec := tokenRecord{
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry,
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeRecord(raw string) (tokenRecord, error) {
	var rec tokenRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return tokenRecord{}, err
	}
	return rec, nil
}

func recordToToken(rec tokenRecord) *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  rec.AccessToken,
		TokenType:    rec.TokenType,
		RefreshToken: rec.RefreshToken,
		Expiry:       rec.Expiry,
	}
}

// LoadOutlookMSALCache reads opaque MSAL cache bytes from keyring (base64). Missing cache returns nil, nil.
func LoadOutlookMSALCache(account string) ([]byte, error) {
	const op = "token.LoadOutlookMSALCache"
	key := OutlookMSALCacheKey(account)
	raw, err := secret.Resolve(key)
	if err != nil {
		if errors.Is(err, secret.ErrNotFound) {
			return nil, nil
		}
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "resolve outlook msal cache").With("key", key)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode outlook msal cache base64").With("key", key)
	}
	return b, nil
}

// SaveOutlookMSALCache persists opaque MSAL cache bytes to keyring (base64).
func SaveOutlookMSALCache(account string, data []byte) error {
	const op = "token.SaveOutlookMSALCache"
	if len(data) == 0 {
		return sirerr.New(sirerr.CodeInvalid, op, "empty msal cache")
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	if err := secret.Set(OutlookMSALCacheKey(account), encoded); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "keyring save outlook msal cache")
	}
	return nil
}

// DeleteOutlookMSALCache removes stored Outlook MSAL material for an account.
func DeleteOutlookMSALCache(account string) error {
	const op = "token.DeleteOutlookMSALCache"
	if err := secret.Delete(OutlookMSALCacheKey(account)); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "delete outlook msal cache")
	}
	return nil
}

// OutlookMSALCacheStatus returns redacted storage status for an account.
func OutlookMSALCacheStatus(account string) string {
	key := OutlookMSALCacheKey(account)
	if v, err := secret.Resolve(key); err == nil && strings.TrimSpace(v) != "" {
		return "(set)"
	}
	return "(unset)"
}

// TokenChanged reports whether refreshed token should be persisted.
func TokenChanged(before, after *oauth2.Token) bool {
	if before == nil || after == nil {
		return true
	}
	if before.AccessToken != after.AccessToken {
		return true
	}
	if before.RefreshToken != after.RefreshToken && after.RefreshToken != "" {
		return true
	}
	if !before.Expiry.Equal(after.Expiry) {
		return true
	}
	return false
}
