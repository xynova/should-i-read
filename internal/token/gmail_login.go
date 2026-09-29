package token

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/browser"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func gmailOAuthConfig(cfg config.Config) (*oauth2.Config, error) {
	const op = "token.gmailOAuthConfig"
	clientID := strings.TrimSpace(cfg.GmailClientID)
	clientSecret := strings.TrimSpace(cfg.GmailClientSecret)
	if clientID == "" {
		return nil, sirerr.New(sirerr.CodeAuth, op, "gmail client id unset; run should-i-read setup")
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{"https://mail.google.com/"},
	}, nil
}

// GmailLogin runs PKCE authorization code flow and saves the token to keyring.
func GmailLogin(ctx context.Context, cfg config.Config, account string) error {
	const op = "token.GmailLogin"
	if ctx == nil {
		return sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	conf, err := gmailOAuthConfig(cfg)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "listen localhost callback")
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	conf.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/oauth2/callback", port)

	state, err := randomState()
	if err != nil {
		return err
	}
	verifier := oauth2.GenerateVerifier()
	authURL := conf.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "consent"),
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/oauth2/callback" {
				http.NotFound(w, r)
				return
			}
			q := r.URL.Query()
			if q.Get("error") != "" {
				errCh <- fmt.Errorf("oauth error: %s", q.Get("error"))
				_, _ = io.WriteString(w, "Authorization failed. You can close this tab.")
				return
			}
			if q.Get("state") != state {
				errCh <- fmt.Errorf("oauth state mismatch")
				_, _ = io.WriteString(w, "State mismatch. You can close this tab.")
				return
			}
			code := q.Get("code")
			if code == "" {
				errCh <- fmt.Errorf("missing authorization code")
				_, _ = io.WriteString(w, "Missing code. You can close this tab.")
				return
			}
			_, _ = io.WriteString(w, "Authorization complete. You can close this tab and return to the terminal.")
			codeCh <- code
		}),
	}

	go func() {
		_ = srv.Serve(ln)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(os.Stderr, "Open this URL in your browser to authorize Gmail:\n%s\n", authURL)
	if err := browser.OpenURL(authURL); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\n", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	var code string
	select {
	case <-waitCtx.Done():
		return sirerr.Wrap(waitCtx.Err(), sirerr.CodeAuth, op, "login timed out waiting for callback")
	case err := <-errCh:
		return sirerr.Wrap(err, sirerr.CodeAuth, op, "oauth callback failed")
	case code = <-codeCh:
	}

	tok, err := conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeAuth, op, "exchange authorization code")
	}
	if tok.RefreshToken == "" && tok.AccessToken == "" {
		return sirerr.New(sirerr.CodeAuth, op, "empty token from google; retry login with prompt=consent")
	}
	if err := SaveGmailToken(account, tok); err != nil {
		return err
	}
	return nil
}

func randomState() (string, error) {
	const op = "token.randomState"
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "generate oauth state")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
