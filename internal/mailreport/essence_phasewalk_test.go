package mailreport

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/behaviorengineering/strop/pkg/integrity"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/config"
)

func TestSemanticRestartPhase_aboutLowRestartsWhy(t *testing.T) {
	email := "GitHub Actions workflow failed"
	about := "Rectangular"
	emb := &vectorStubEmbedder{byText: map[string][]float64{
		email: {1, 0, 0},
		about: {0, 1, 0},
	}}
	e := &polypusEssencer{
		embedder:              emb,
		descriptionCosineMode: config.DescriptionCosineModeRepair,
		descriptionCosineMin:  0.55,
	}
	restart := semanticRestartPhase(context.Background(), e, email, harness.EssenceOut{
		About: about,
		Shape: "ci failure notification",
	})
	if restart != essencePhaseWhy {
		t.Fatalf("restart %q", restart)
	}
}

func TestSemanticRestartPhase_shapeEmailCosineDoesNotRestartTags(t *testing.T) {
	email := "Couldn't process payment. Insufficient funds on the card."
	about := "Notification of failed payment processing due to insufficient funds"
	shape := "billing payment-failure"
	emb := &vectorStubEmbedder{byText: map[string][]float64{
		email: {1, 0, 0},
		about: {0.98, 0.02, 0},
		shape: {0, 1, 0},
	}}
	e := &polypusEssencer{
		embedder:              emb,
		descriptionCosineMode: config.DescriptionCosineModeRepair,
		descriptionCosineMin:  0.55,
	}
	restart := semanticRestartPhase(context.Background(), e, email, harness.EssenceOut{
		About: about,
		Shape: shape,
	})
	if restart != "" {
		t.Fatalf("SHAPE vs email cosine restarted %q", restart)
	}
}

func TestKeepBetterText_rejectsLowerScore(t *testing.T) {
	kept, score := keepBetterText("due to insufficient funds", 0.54, "failed payment", 0.40)
	if kept != "due to insufficient funds" || score != 0.54 {
		t.Fatalf("kept %q score %v", kept, score)
	}
}

func TestSemanticRestartFeedback_includesAboutScore(t *testing.T) {
	t.Setenv(benchEnvFeedback, "")
	got := semanticRestartFeedback("ABOUT", 0.49, 0.55)
	if !strings.Contains(got, "0.49") || !strings.Contains(got, "ABOUT") {
		t.Fatalf("feedback %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "preserve") {
		t.Fatalf("feedback missing preserve guidance: %q", got)
	}
	lower := strings.ToLower(got)
	if !strings.Contains(lower, "silently validate the candidate") ||
		!strings.Contains(lower, "check every name, amount, version, service, and cause") ||
		!strings.Contains(lower, "specific event") ||
		!strings.Contains(lower, "do not output this checklist or your reasoning") {
		t.Fatalf("feedback missing silent validation checklist: %q", got)
	}
}

func TestEssencePhaseWalk_repairPassesFeedbackAndKeepsBetterAbout(t *testing.T) {
	email := "Couldn't process payment. Insufficient funds on the card."
	firstAbout := "Notification of failed payment processing due to insufficient funds"
	worseAbout := "A message"
	emb := &vectorStubEmbedder{byText: map[string][]float64{
		email:      {1, 0, 0},
		firstAbout: {0.50, 0.866, 0},
		worseAbout: {0, 1, 0},
	}}
	whyChat := &scriptedEssenceChat{replies: []string{
		essenceWhyRaw(firstAbout),
		essenceWhyRaw(worseAbout),
	}}
	tagsChat := &scriptedEssenceChat{replies: []string{"TAGS: billing payment-failure"}}
	kindChat := &scriptedEssenceChat{replies: []string{"KIND: notification > billing > payment-failure"}}
	whyShell := mustEssenceTestShell(t, whyChat, essenceWhyShellHooks(nil))
	tagsShell := mustEssenceTestShell(t, tagsChat, essenceTagsShellHooks(nil))
	kindShell := mustEssenceTestShell(t, kindChat, essenceKindShellHooks(nil))
	e := &polypusEssencer{
		whyShell:                   whyShell,
		tagsShell:                  tagsShell,
		kindShell:                  kindShell,
		whyEscalateShell:           whyShell,
		tagsEscalateShell:          tagsShell,
		kindEscalateShell:          kindShell,
		embedder:                   emb,
		descriptionCosineMode:      config.DescriptionCosineModeRepair,
		descriptionCosineMin:       0.55,
		essenceSemanticMaxAttempts: 2,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := e.essencePhaseWalk(ctx, email, "World\n\nMessage:\n"+email, BuildEssencePrompts(nil))
	if err != nil {
		t.Fatal(err)
	}
	if out.About != firstAbout {
		t.Fatalf("about %q want first candidate", out.About)
	}
	if len(whyChat.users) < 2 {
		t.Fatalf("why users %d want rewind", len(whyChat.users))
	}
	if !strings.Contains(whyChat.users[1], "ABOUT") || !strings.Contains(whyChat.users[1], "0.") {
		t.Fatalf("second why user missing repair feedback: %q", whyChat.users[1])
	}
}

func TestEssencePhaseWalk_semanticFailureUsesEscalationModelOnce(t *testing.T) {
	email := "Couldn't process payment. Insufficient funds on the card."
	weakAbout := "Notification of a failed payment attempt"
	strongAbout := "Notification of a failed payment due to insufficient funds"
	emb := &vectorStubEmbedder{byText: map[string][]float64{
		email:       {1, 0},
		weakAbout:   {0.50, 0.866},
		strongAbout: {0.90, 0.436},
	}}
	primaryWhy := &scriptedEssenceChat{replies: []string{essenceWhyRaw(weakAbout)}}
	escalatedWhy := &scriptedEssenceChat{replies: []string{essenceWhyRaw(strongAbout)}}
	primaryTags := &scriptedEssenceChat{replies: []string{"TAGS: billing payment-failure"}}
	escalatedTags := &scriptedEssenceChat{replies: []string{"TAGS: billing payment-failure"}}
	primaryKind := &scriptedEssenceChat{replies: []string{"KIND: notification > billing > payment-failure"}}
	escalatedKind := &scriptedEssenceChat{replies: []string{"KIND: notification > billing > payment-failure"}}
	e := &polypusEssencer{
		whyShell:                   mustEssenceTestShell(t, primaryWhy, essenceWhyShellHooks(nil)),
		tagsShell:                  mustEssenceTestShell(t, primaryTags, essenceTagsShellHooks(nil)),
		kindShell:                  mustEssenceTestShell(t, primaryKind, essenceKindShellHooks(nil)),
		whyEscalateShell:           mustEssenceTestShell(t, escalatedWhy, essenceWhyShellHooks(nil)),
		tagsEscalateShell:          mustEssenceTestShell(t, escalatedTags, essenceTagsShellHooks(nil)),
		kindEscalateShell:          mustEssenceTestShell(t, escalatedKind, essenceKindShellHooks(nil)),
		embedder:                   emb,
		escalateModel:              "strong-model",
		descriptionCosineMode:      config.DescriptionCosineModeRepair,
		descriptionCosineMin:       0.55,
		essenceSemanticMaxAttempts: 1,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := e.essencePhaseWalk(ctx, email, "World\n\nMessage:\n"+email, BuildEssencePrompts(nil))
	if err != nil {
		t.Fatal(err)
	}
	if out.About != strongAbout {
		t.Fatalf("about %q want escalated candidate %q", out.About, strongAbout)
	}
	if len(escalatedWhy.users) != 1 || !strings.Contains(escalatedWhy.users[0], "ABOUT") {
		t.Fatalf("escalation feedback not sent: %#v", escalatedWhy.users)
	}
	if len(escalatedTags.users) != 1 || len(escalatedKind.users) != 1 {
		t.Fatalf("dependent escalation calls tags=%d kind=%d", len(escalatedTags.users), len(escalatedKind.users))
	}
}

func essenceWhyRaw(about string) string {
	return "WHY1: The message reports a failed payment.\nWHY2: The card was declined.\nWHY3: The account had insufficient funds.\nWHY4: The subscription retry failed.\nWHY5: Billing could not complete.\nABOUT: " + about
}

type scriptedEssenceChat struct {
	mu      sync.Mutex
	replies []string
	users   []string
	n       int
}

func (s *scriptedEssenceChat) Chat(_ context.Context, messages []integrity.Message) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range messages {
		if m.Role == integrity.RoleUser {
			s.users = append(s.users, m.Content)
		}
	}
	if len(s.replies) == 0 {
		return "", nil
	}
	i := s.n
	if i >= len(s.replies) {
		i = len(s.replies) - 1
	}
	s.n++
	return s.replies[i], nil
}

func mustEssenceTestShell(t *testing.T, chat *scriptedEssenceChat, hooks essenceShellHooks) *integrity.Shell {
	t.Helper()
	shell, err := integrity.CreateShell(integrity.ShellConfig{
		Chat:         chat.Chat,
		Validate:     hooks.Validate,
		CompleteNote: hooks.CompleteNote,
		AfterMiss:    hooks.AfterMiss,
		MaxAttempts:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return shell
}
