package configure

import (
	"errors"
	"strings"
	"testing"

	"github.com/xynova/should-i-read/internal/mailsync"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestFormatChecklist_noRedundantDetail(t *testing.T) {
	t.Parallel()
	snap := Snapshot{
		Mail: mailsync.Status{Email: "me@example.com"},
		Steps: []StepStatus{
			{ID: StepHostConfig, OK: true, Detail: "Operator config is present."},
			{ID: StepLocalStore, OK: false, Detail: "Local mail store is not initialized."},
		},
	}
	out := FormatChecklist(snap)
	if strings.Contains(out, "Operator config is present") {
		t.Fatalf("ok steps should not repeat detail prose: %q", out)
	}
	if !strings.Contains(out, "me@example.com") {
		t.Fatalf("expected saved email in header: %q", out)
	}
	if !strings.Contains(out, "not initialized") {
		t.Fatalf("expected short missing hint: %q", out)
	}
}

func TestNeedsMailboxPrompt_skipsWhenProfileSaved(t *testing.T) {
	t.Parallel()
	snap := Snapshot{
		Mail: mailsync.Status{
			ConfigOK: true,
			Provider: "gmail",
			Email:    "me@example.com",
			Account:  "gmail",
		},
	}
	opts := Options{}
	applyMailFromSnapshot(&opts, snap)
	if needsMailboxPrompt(StepLocalStore, opts, snap) {
		t.Fatal("should not prompt for email when mailbox profile is saved")
	}
	if needsMailboxPrompt(StepMailboxLogin, opts, snap) {
		t.Fatal("should not prompt for login step when profile is saved")
	}
}

func TestNeedsMailboxPrompt_asksWhenProfileMissing(t *testing.T) {
	t.Parallel()
	snap := Snapshot{Mail: mailsync.Status{ConfigOK: false}}
	opts := Options{}
	if !needsMailboxPrompt(StepMailboxProfile, opts, snap) {
		t.Fatal("expected prompt when profile missing")
	}
}

func TestNeedsMailboxPrompt_configOKEmptyEmailTokenOK(t *testing.T) {
	t.Parallel()
	snap := Snapshot{
		Mail: mailsync.Status{
			ConfigOK: true,
			TokenOK:  true,
			Provider: "gmail",
			Account:  "gmail",
		},
	}
	opts := Options{Provider: "gmail", Account: "gmail"}
	if needsMailboxPrompt(StepLocalStore, opts, snap) {
		t.Fatal("should not prompt when ConfigOK+TokenOK with provider and account")
	}
}

func TestNeedsMailboxPrompt_skipsProfileWhenConfigOK(t *testing.T) {
	t.Parallel()
	snap := Snapshot{Mail: mailsync.Status{ConfigOK: true}}
	opts := Options{}
	if needsMailboxPrompt(StepMailboxProfile, opts, snap) {
		t.Fatal("should not re-prompt mailbox profile when ConfigOK")
	}
}

func TestHumanFailure_usesStderr(t *testing.T) {
	t.Parallel()
	err := sirerr.Wrap(errors.New("exit status 1"), sirerr.CodeFailed, "pimalaya.RunJSON", "pimalaya CLI failed").
		With("stderr", "folder INBOX does not exist\nmore detail")
	wrapped := sirerr.Wrap(err, sirerr.CodeFailed, "configure.runStepBody", "Local mail store setup failed.")
	msg, hint := humanFailure(wrapped)
	if !strings.Contains(msg, "folder INBOX") {
		t.Fatalf("expected stderr in msg, got %q", msg)
	}
	if hint == "" {
		t.Fatal("expected reuse hint")
	}
}

func TestHumanFailure_prefersStoreMessageWithoutStderr(t *testing.T) {
	t.Parallel()
	err := sirerr.Wrap(errors.New("exit status 1"), sirerr.CodeFailed, "pimalaya.RunJSON", "pimalaya CLI failed")
	wrapped := sirerr.Wrap(err, sirerr.CodeFailed, "configure.runStepBody", "Local mail store setup failed.")
	msg, _ := humanFailure(wrapped)
	if !strings.Contains(msg, "Local mail store setup failed") {
		t.Fatalf("got %q", msg)
	}
}

func TestFormatHubSummary_ready(t *testing.T) {
	t.Parallel()
	s := FormatHubSummary(Snapshot{Ready: true})
	if !strings.Contains(s, "Ready.") {
		t.Fatalf("got %q", s)
	}
}
