package configure

import (
	"github.com/xynova/should-i-read/internal/mailsync"
)

const (
	StepHostConfig     = "host_config"
	StepOAuthClients   = "oauth_clients"
	StepMailDependency = "mail_dependency"
	StepMailboxProfile = "mailbox_profile"
	StepMailboxLogin   = "mailbox_login"
	StepLocalStore     = "local_store"
)

const WaitingOnMailboxLogin = "mailbox_login"

// StepOrder is the canonical order for RunAllMissing.
var StepOrder = []string{
	StepHostConfig,
	StepOAuthClients,
	StepMailDependency,
	StepMailboxProfile,
	StepMailboxLogin,
	StepLocalStore,
}

// StepStatus is one row in the configure checklist.
type StepStatus struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Snapshot is operator configure readiness.
type Snapshot struct {
	Ready       bool              `json:"ready"`
	Steps       []StepStatus      `json:"steps"`
	Mail        mailsync.Status   `json:"mail"`
	OAuth       map[string]string `json:"oauth_status"`
	WaitingOn   string            `json:"waiting_on,omitempty"`
	NextActions []string          `json:"next_actions,omitempty"`
}

// StepResult records one executed step.
type StepResult struct {
	Step    string `json:"step"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// SessionResult is returned from hub, apply, or single step runs.
type SessionResult struct {
	Snapshot Snapshot     `json:"snapshot"`
	Ran      []StepResult `json:"ran,omitempty"`
}

// MailboxPrefill holds mailbox fields for prompts.
type MailboxPrefill struct {
	Provider string
	Email    string
	Account  string
}

// Options drives configure runs.
type Options struct {
	Provider    string
	Email       string
	Account     string
	StoreRoot   string
	Force       bool
	SkipLogin   bool
	Interactive bool
	Apply       bool
	JSONOnly    bool
	Step        string
}
