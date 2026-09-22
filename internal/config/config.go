package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultPolypusBaseURL = "http://127.0.0.1:1320"
	emailopsAppID         = "com.emailops.app"
)

// Config holds host operator settings for EmailOps and Polypus.
type Config struct {
	PolypusBaseURL  string
	EmailOpsDataDir string
	EmailOpsCLI     string
	RepoRoot        string
}

// Create loads Config from environment with platform defaults.
// EMAILOPS_DATA_DIR / --data-dir callers override EmailOpsDataDir after Create.
func Create(repoRoot string) Config {
	root := strings.TrimSpace(repoRoot)
	cfg := Config{
		PolypusBaseURL:  envOr("POLYPUS_BASE_URL", defaultPolypusBaseURL),
		EmailOpsDataDir: strings.TrimSpace(os.Getenv("EMAILOPS_DATA_DIR")),
		EmailOpsCLI:     strings.TrimSpace(os.Getenv("EMAILOPS_CLI")),
		RepoRoot:        root,
	}
	if cfg.EmailOpsDataDir == "" {
		cfg.EmailOpsDataDir = defaultEmailOpsDataDir()
	}
	cfg.PolypusBaseURL = strings.TrimRight(cfg.PolypusBaseURL, "/")
	return cfg
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func defaultEmailOpsDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", emailopsAppID)
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, emailopsAppID)
		}
		return filepath.Join(home, "AppData", "Roaming", emailopsAppID)
	default:
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, emailopsAppID)
		}
		return filepath.Join(home, ".local", "share", emailopsAppID)
	}
}
