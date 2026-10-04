package configure

import (
	"context"
	"testing"

	"github.com/xynova/should-i-read/internal/mailsync"
)

type stubHubPrompter struct {
	collectCalls int
}

func (s *stubHubPrompter) ChooseAction(context.Context, Snapshot) (HubAction, error) {
	return HubFixStep, nil
}

func (s *stubHubPrompter) ChooseStep(context.Context, Snapshot) (string, error) {
	return StepLocalStore, nil
}

func (s *stubHubPrompter) ConfirmBrowserLogin(context.Context) (bool, error) {
	return false, nil
}

func (s *stubHubPrompter) ConfirmForceOverwrite(context.Context, string) (bool, error) {
	return false, nil
}

func (s *stubHubPrompter) CollectMailboxFields(context.Context, MailboxPrefill) (MailboxPrefill, error) {
	s.collectCalls++
	return MailboxPrefill{}, nil
}

func TestNeedsMailboxPrompt_gateMatchesHubFixStep(t *testing.T) {
	t.Parallel()
	snap := Snapshot{
		Mail: mailsync.Status{
			ConfigOK: true,
			TokenOK:  true,
			Provider: "gmail",
			Email:    "me@example.com",
			Account:  "gmail",
		},
	}
	opts := Options{}
	applyMailFromSnapshot(&opts, snap)
	if needsMailboxPrompt(StepLocalStore, opts, snap) {
		t.Fatal("FixStep must not call CollectMailboxFields for this snapshot")
	}
}

func TestApplyMailFromSnapshot_fillsEmptyOpts(t *testing.T) {
	t.Parallel()
	opts := Options{}
	applyMailFromSnapshot(&opts, Snapshot{
		Mail: mailsync.Status{Provider: "gmail", Email: "a@b.com", Account: "gmail"},
	})
	if opts.Email != "a@b.com" || opts.Provider != "gmail" {
		t.Fatalf("got %+v", opts)
	}
}

var _ HubPrompter = (*stubHubPrompter)(nil)
