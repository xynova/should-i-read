package configure

import "context"

// HubAction is a hub menu choice.
type HubAction string

const (
	HubRunMissing HubAction = "run_missing"
	HubFixStep    HubAction = "fix_step"
	HubQuit       HubAction = "quit"
)

// HubPrompter drives the TTY configure hub.
type HubPrompter interface {
	ChooseAction(ctx context.Context, snap Snapshot) (HubAction, error)
	ChooseStep(ctx context.Context, snap Snapshot) (stepID string, err error)
	ConfirmBrowserLogin(ctx context.Context) (bool, error)
	ConfirmForceOverwrite(ctx context.Context, path string) (bool, error)
	CollectMailboxFields(ctx context.Context, prefill MailboxPrefill) (MailboxPrefill, error)
}
