package mailsync

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/pimalaya"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// Status is mailbox readiness in product language.
type Status struct {
	Ready          bool     `json:"ready"`
	DepOK          bool     `json:"dep_ok"`
	ConfigOK       bool     `json:"config_ok"`
	TokenOK        bool     `json:"token_ok"`
	StoreOK        bool     `json:"store_ok"`
	Provider       string   `json:"provider,omitempty"`
	Account        string   `json:"account,omitempty"`
	Email          string   `json:"email,omitempty"`
	MailSyncConfig string   `json:"mail_sync_config,omitempty"`
	PimdirPath     string   `json:"pimdir_path,omitempty"`
	Messages       []string `json:"messages"`
	NextSteps      []string `json:"next_steps,omitempty"`
}

// CollectStatus probes filesystem and keyring without running a full sync check.
func CollectStatus(ctx context.Context, cfg config.Config) (Status, error) {
	const op = "mailsync.CollectStatus"
	if ctx == nil {
		return Status{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	st := Status{Messages: []string{}}
	st.MailSyncConfig = strings.TrimSpace(cfg.Pimalaya.NeverestConfig)
	st.PimdirPath = strings.TrimSpace(cfg.Pimalaya.PimdirPath)
	st.Account = strings.TrimSpace(cfg.Pimalaya.DefaultAccount)

	if _, err := pimalaya.ResolveBin(cfg.Pimalaya.NeverestBin, "neverest"); err == nil {
		st.DepOK = true
	} else {
		st.Messages = append(st.Messages, "Mail sync dependency is not installed; run should-i-read configure.")
	}

	mailSyncPath, err := config.DefaultMailSyncConfigPath()
	if err == nil && st.MailSyncConfig == "" {
		st.MailSyncConfig = mailSyncPath
	}

	configOK := st.MailSyncConfig != "" && st.PimdirPath != ""
	if configOK {
		if _, err := os.Stat(st.MailSyncConfig); err != nil {
			configOK = false
		}
	}
	st.ConfigOK = configOK
	if !st.ConfigOK {
		st.Messages = append(st.Messages, "Mailbox is not configured; run should-i-read configure.")
	} else if raw, err := os.ReadFile(st.MailSyncConfig); err == nil {
		body := string(raw)
		if st.Email = parseEmailFromMailSyncTOML(body); st.Email != "" {
			// ok
		}
		if acct := parseAccountFromMailSyncTOML(body); acct != "" && st.Account == "" {
			st.Account = acct
		}
		if prov := InferProviderFromMailSyncTOML(body); prov != "" {
			st.Provider = string(prov)
		}
	}

	provider := Provider(st.Provider)
	if provider == "" {
		provider = inferProvider(st.Account)
		st.Provider = string(provider)
	}

	if provider != "" && oauthTokenReady(provider, st.Account) {
		st.TokenOK = true
	} else if st.ConfigOK {
		st.Messages = append(st.Messages, "Mailbox login is not complete; run should-i-read configure and fix the mailbox login step.")
	}

	if st.PimdirPath != "" {
		dbPath := filepath.Join(st.PimdirPath, "pimdir.db")
		if _, err := os.Stat(dbPath); err == nil {
			st.StoreOK = true
		} else if st.ConfigOK && st.TokenOK {
			st.Messages = append(st.Messages, "Local mail store is not initialized; run should-i-read configure.")
		}
	}

	st.Ready = st.DepOK && st.ConfigOK && st.TokenOK && st.StoreOK
	if !st.Ready {
		st.NextSteps = productNextSteps(st)
	}
	return st, nil
}

func inferProvider(account string) Provider {
	switch strings.ToLower(strings.TrimSpace(account)) {
	case string(ProviderOutlook):
		return ProviderOutlook
	case string(ProviderGmail):
		return ProviderGmail
	default:
		return ""
	}
}

func productNextSteps(st Status) []string {
	if !st.DepOK || !st.ConfigOK {
		return []string{"should-i-read configure  # or: configure --apply --provider gmail --email you@example.com"}
	}
	if !st.TokenOK {
		return []string{"should-i-read configure  # fix mailbox login step"}
	}
	if !st.StoreOK {
		return []string{"should-i-read configure  # finish local mail store step"}
	}
	return nil
}
