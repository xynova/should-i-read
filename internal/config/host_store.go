package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const hostStoreFileName = "should-i-read.db"

// DefaultHostStorePath returns the platform data directory for the classify ledger.
func DefaultHostStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", AppName, "host", hostStoreFileName), nil
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, AppName, "host", hostStoreFileName), nil
		}
		return filepath.Join(home, "AppData", "Local", AppName, "host", hostStoreFileName), nil
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, AppName, "host", hostStoreFileName), nil
		}
		return filepath.Join(home, ".local", "share", AppName, "host", hostStoreFileName), nil
	}
}
