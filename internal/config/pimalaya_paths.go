package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const mailSyncConfigName = "mail-sync.toml"

// DefaultMailSyncConfigPath returns ~/.config/should-i-read/mail-sync.toml (or XDG equivalent).
func DefaultMailSyncConfigPath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, mailSyncConfigName), nil
}

// DefaultPimStoreDir returns the platform data directory for a pimdir account store.
func DefaultPimStoreDir(account string) string {
	acct := sanitizeAccountSegment(account)
	if acct == "" {
		acct = "default"
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", AppName, "pim", acct)
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, AppName, "pim", acct)
		}
		return filepath.Join(home, "AppData", "Local", AppName, "pim", acct)
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, AppName, "pim", acct)
		}
		return filepath.Join(home, ".local", "share", AppName, "pim", acct)
	}
}

func sanitizeAccountSegment(account string) string {
	s := strings.TrimSpace(account)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
