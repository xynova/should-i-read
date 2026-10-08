package configure

import (
	"context"
	"os"
	"strings"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/mailsync"
	"github.com/xynova/should-i-read/internal/oauthcred"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// CollectSnapshot probes host, oauth, and mail readiness.
func CollectSnapshot(ctx context.Context, repoRoot string, cfg config.Config) (Snapshot, error) {
	const op = "configure.CollectSnapshot"
	if ctx == nil {
		return Snapshot{}, sirerr.New(sirerr.CodeInvalid, op, "nil context")
	}
	mail, err := mailsync.CollectStatus(ctx, cfg)
	if err != nil {
		return Snapshot{}, err
	}
	oauth := oauthcred.Status()
	snap := Snapshot{
		Mail:  mail,
		OAuth: oauth,
		Steps: make([]StepStatus, 0, len(StepOrder)),
	}
	for _, id := range StepOrder {
		ok, detail := probeStep(id, repoRoot, cfg, mail, oauth)
		snap.Steps = append(snap.Steps, StepStatus{ID: id, OK: ok, Detail: detail})
	}
	snap.Ready = allStepsOK(snap.Steps)
	if !snap.Ready && !mail.TokenOK && mail.ConfigOK {
		snap.WaitingOn = WaitingOnMailboxLogin
	}
	if snap.Ready {
		snap.NextActions = []string{
			"./bin/should-i-read config bump  # when Polypus URL is still literal localhost",
			"make polypus-check",
			"make readiness && make sync && make export",
		}
	} else if len(mail.NextSteps) > 0 {
		snap.NextActions = mail.NextSteps
	}
	return snap, nil
}

func allStepsOK(steps []StepStatus) bool {
	for _, s := range steps {
		if !s.OK {
			return false
		}
	}
	return true
}

func probeStep(id, repoRoot string, cfg config.Config, mail mailsync.Status, oauth map[string]string) (bool, string) {
	switch id {
	case StepHostConfig:
		path, err := config.ResolvePath("")
		if err != nil || path == "" {
			return false, "Operator config is missing."
		}
		if _, err := os.Stat(path); err != nil {
			return false, "Operator config file is missing."
		}
		return true, "Operator config is present."
	case StepOAuthClients:
		if oauthClientsOK(mail, oauth) {
			return true, "OAuth app credentials are available."
		}
		return false, "OAuth app credentials are missing; fix this step or run should-i-read configure."
	case StepMailDependency:
		if mail.DepOK {
			return true, "Mail sync dependency is available."
		}
		return false, "Mail sync dependency is not installed."
	case StepMailboxProfile:
		if mail.ConfigOK {
			return true, "Mailbox settings are saved."
		}
		return false, "Mailbox settings are not saved."
	case StepMailboxLogin:
		if mail.TokenOK {
			return true, "Mailbox login is stored."
		}
		return false, "Mailbox login is not complete."
	case StepLocalStore:
		if mail.StoreOK {
			return true, "Local mail store is ready."
		}
		return false, "Local mail store is not initialized."
	default:
		return false, "Unknown step."
	}
}

func oauthClientsOK(mail mailsync.Status, oauth map[string]string) bool {
	prov := strings.ToLower(strings.TrimSpace(mail.Provider))
	switch prov {
	case string(mailsync.ProviderGmail):
		return oauth[oauthcred.EnvGmailClientID] == "(set)"
	case string(mailsync.ProviderOutlook):
		return oauth[oauthcred.EnvOutlookClientID] == "(set)"
	default:
		return oauthcred.HasAny()
	}
}

func stepOK(snap Snapshot, id string) bool {
	for _, s := range snap.Steps {
		if s.ID == id {
			return s.OK
		}
	}
	return false
}
