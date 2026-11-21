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
	Ready            bool                `json:"ready"`
	DepOK            bool                `json:"dep_ok"`
	ConfigOK         bool                `json:"config_ok"`
	TokenOK          bool                `json:"token_ok"`
	StoreOK          bool                `json:"store_ok"`
	HimalayaConfigOK bool                `json:"himalaya_config_ok"`
	HimalayaConfig   string              `json:"himalaya_config,omitempty"`
	Provider         string              `json:"provider,omitempty"`
	Account          string              `json:"account,omitempty"`
	Email            string              `json:"email,omitempty"`
	MailSyncConfig   string              `json:"mail_sync_config,omitempty"`
	PimdirPath       string              `json:"pimdir_path,omitempty"`
	Messages         []string            `json:"messages"`
	NextSteps        []string            `json:"next_steps,omitempty"`
	ClassifyStore    ClassifyStoreStatus `json:"classify_store,omitempty"`
}

// ClassifyStoreStatus is host ledger counts for mail status JSON.
type ClassifyStoreStatus struct {
	Path                 string `json:"path,omitempty"`
	Complete             int    `json:"complete"`
	Error                int    `json:"error"`
	Running              int    `json:"running"`
	Pending              int    `json:"pending"`
	OutcomesTrashed      int    `json:"outcomes_trashed,omitempty"`
	OutcomesAlreadyTrash int    `json:"outcomes_already_in_trash,omitempty"`
	OutcomesKept         int    `json:"outcomes_kept,omitempty"`
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
	st.HimalayaConfig = strings.TrimSpace(cfg.Pimalaya.HimalayaConfig)
	st.Account = strings.TrimSpace(cfg.Pimalaya.DefaultAccount)
	st.fillDep(cfg)
	st.fillConfig()
	st.fillToken()
	st.fillStore()
	st.fillHimalaya()
	st.Ready = st.DepOK && st.ConfigOK && st.TokenOK && st.StoreOK
	if !st.Ready {
		st.NextSteps = productNextSteps(st)
	}
	return st, nil
}

func (st *Status) fillDep(cfg config.Config) {
	if _, err := pimalaya.ResolveBin(cfg.Pimalaya.NeverestBin, "neverest"); err == nil {
		st.DepOK = true
		return
	}
	st.Messages = append(st.Messages, "Mail sync dependency is not installed; run should-i-read configure.")
}

func (st *Status) fillConfig() {
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
		return
	}
	raw, err := os.ReadFile(st.MailSyncConfig)
	if err != nil {
		return
	}
	body := string(raw)
	st.Email = parseEmailFromMailSyncTOML(body)
	if acct := parseAccountFromMailSyncTOML(body); acct != "" && st.Account == "" {
		st.Account = acct
	}
	if prov := InferProviderFromMailSyncTOML(body); prov != "" {
		st.Provider = string(prov)
	}
}

func (st *Status) fillToken() {
	provider := Provider(st.Provider)
	if provider == "" {
		provider = inferProvider(st.Account)
		st.Provider = string(provider)
	}
	if provider != "" && oauthTokenReady(provider, st.Account) {
		st.TokenOK = true
		return
	}
	if st.ConfigOK {
		st.Messages = append(st.Messages, "Mailbox login is not complete; run should-i-read configure and fix the mailbox login step.")
	}
}

func (st *Status) fillStore() {
	if st.PimdirPath == "" {
		return
	}
	dbPath := filepath.Join(st.PimdirPath, "pimdir.db")
	if _, err := os.Stat(dbPath); err == nil {
		st.StoreOK = true
		return
	}
	if st.ConfigOK && st.TokenOK {
		st.Messages = append(st.Messages, "Local mail store is not initialized; run should-i-read configure.")
	}
}

func (st *Status) fillHimalaya() {
	himalayaPath := st.HimalayaConfig
	if himalayaPath == "" {
		p, perr := config.DefaultHimalayaConfigPath()
		if perr != nil {
			st.Messages = append(st.Messages, "Himalaya pimdir config path is unset; run should-i-read configure.")
			return
		}
		himalayaPath = p
		st.HimalayaConfig = p
	}
	if fi, err := os.Stat(himalayaPath); err == nil && !fi.IsDir() {
		st.HimalayaConfigOK = true
		return
	}
	st.Messages = append(st.Messages, "Himalaya pimdir config is missing; run should-i-read configure --step mailbox_profile.")
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
