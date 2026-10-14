package mailreport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/polypus"
	"github.com/xynova/should-i-read/internal/sirerr"
)

func TestRollupAttentionReportFixtures(t *testing.T) {
	t.Parallel()
	reminder := MessageRow{
		ObjectHash: "oi2mp4tfpy4qqdnufd5o42cx7gxjmug56y2jviii5vhz52nbfxoq",
		Attention:  &Attention{Urgency: 0.82, NeedsReply: false, InvolvesDecision: true, NeedsReplyNoul: 0.10, InvolvesDecisionNoul: 0.91},
	}
	seek := MessageRow{
		ObjectHash: "235rdq4lialgfg46pymmjcpmwwd2s3xyduvd2ugzmirmrlxmk5sq",
		Attention:  &Attention{Urgency: 0.05, NeedsReply: false, InvolvesDecision: false, NeedsReplyNoul: 0.05, InvolvesDecisionNoul: 0.05},
	}
	pr := MessageRow{
		ObjectHash: "tbkt6ub6v2cjnfz2tqdc5pq32hvw3jp46n4hk5sl3jzc7r7ugmoq",
		Attention:  &Attention{Urgency: 0.18, NeedsReply: false, InvolvesDecision: false, NeedsReplyNoul: 0.08, InvolvesDecisionNoul: 0.08},
	}
	failMembers := []MessageRow{
		{ObjectHash: "a", Attention: &Attention{Urgency: 0.45, InvolvesDecision: true, NeedsReplyNoul: 0.05, InvolvesDecisionNoul: 0.55}},
		{ObjectHash: "bou2643wi3tvbrdv4lpmdjaqbq374hhra6ybedk7w43kzy4rjnia", Attention: &Attention{Urgency: 0.20, NeedsReplyNoul: 0.05, InvolvesDecisionNoul: 0.05}},
	}
	failRoll := RollupAttention(failMembers, 0.5)
	if failRoll == nil || failRoll.Urgency != 0.45 || !failRoll.InvolvesDecision || failRoll.NeedsReply {
		t.Fatalf("failure rollup: %+v", failRoll)
	}
	remRoll := RollupAttention([]MessageRow{reminder}, 0.5)
	if remRoll == nil || remRoll.Urgency != 0.82 || !remRoll.InvolvesDecision {
		t.Fatalf("reminder: %+v", remRoll)
	}
	seekRoll := RollupAttention([]MessageRow{seek}, 0.5)
	if seekRoll == nil || seekRoll.Urgency != 0.05 || seekRoll.InvolvesDecision {
		t.Fatalf("seek: %+v", seekRoll)
	}
	_ = RollupAttention([]MessageRow{pr}, 0.5)
	if remRoll.Urgency <= failRoll.Urgency {
		t.Fatalf("reminder should outrank failure cluster max")
	}
}

func TestAttentionTrueCut(t *testing.T) {
	t.Parallel()
	a, err := parseAttentionAnswers("test", testSystemOneAnswers(0.5, 0.5, 0.5), 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if !a.NeedsReply {
		t.Fatal("expected needs_reply true at boundary")
	}
}

func testSystemOneAnswers(u, r, d float64) polypus.SystemOneResponse {
	return polypus.SystemOneResponse{
		Answers: map[string]polypus.SystemOneAnswer{
			"urgency":           {Type: "noul", Noul: u},
			"needs_reply":       {Type: "noul", Noul: r},
			"involves_decision": {Type: "noul", Noul: d},
		},
	}
}

func TestAttentionJSONOmitUnscored(t *testing.T) {
	t.Parallel()
	unscored, err := json.Marshal(ClusterRow{ID: "c1", SuggestedAction: SuggestedReview})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(unscored), "attention") {
		t.Fatalf("unscored: %s", unscored)
	}
	scored, err := json.Marshal(ClusterRow{
		ID:              "c2",
		SuggestedAction: SuggestedReview,
		Attention:       &Attention{Urgency: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(scored), `"urgency":0`) || !strings.Contains(string(scored), `"needs_reply":false`) {
		t.Fatalf("scored zero urgency: %s", scored)
	}
}

func TestSortClustersByAttention(t *testing.T) {
	t.Parallel()
	clusters := []ClusterRow{
		{ID: "c-fail", Size: 11, Attention: &Attention{Urgency: 0.45}},
		{ID: "c-rem", Size: 1, Attention: &Attention{Urgency: 0.82, InvolvesDecision: true}},
	}
	SortClustersByAttention(clusters)
	if clusters[0].ID != "c-rem" {
		t.Fatalf("order: %+v", clusters)
	}
}

func TestAttentionScorer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		body, err := decodeSystemOneBody(r)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		state, _ := body["state"].(string)
		if !strings.Contains(state, "Subject:") || !strings.Contains(state, "About:") {
			http.Error(w, "bad state", 400)
			return
		}
		questions, _ := body["questions"].(map[string]any)
		if questions == nil || questions["urgency"] == nil {
			http.Error(w, "bad questions", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "jev",
			"answers": map[string]any{
				"urgency":           map[string]any{"type": "noul", "noul": 1.4},
				"needs_reply":       map[string]any{"type": "noul", "noul": 0.5},
				"involves_decision": map[string]any{"type": "noul", "noul": 0.1},
			},
		})
	}))
	defer srv.Close()
	c, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	scorer, err := CreateAttentionScorer(AttentionScorerConfig{Client: c, Model: "jev", TrueMin: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := scorer.Score(ctx, AttentionIn{
		Subject: "Action required",
		Sender:  "GitLab",
		About:   "quota",
		Kind:    "reminder",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Urgency != 1 || !got.NeedsReply {
		t.Fatalf("got %+v", got)
	}
}

func TestAttentionScorerMissingAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				"urgency":     map[string]any{"type": "noul", "noul": 0.5},
				"needs_reply": map[string]any{"type": "noul", "noul": 0.5},
			},
		})
	}))
	defer srv.Close()
	c, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	scorer, _ := CreateAttentionScorer(AttentionScorerConfig{Client: c, Model: "jev"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = scorer.Score(ctx, AttentionIn{Subject: "x", About: "y"})
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeAI {
		t.Fatalf("code: %v", err)
	}
}

func TestAttentionScorer429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	scorer, _ := CreateAttentionScorer(AttentionScorerConfig{Client: c, Model: "jev"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = scorer.Score(ctx, AttentionIn{Subject: "x", About: "y"})
	if err == nil {
		t.Fatal("expected error")
	}
	code, ok := sirerr.AsCode(err)
	if !ok || code != sirerr.CodeUnavailable {
		t.Fatalf("code: %v", err)
	}
}

func TestAttentionScorerNilContext(t *testing.T) {
	c, err := polypus.Create("http://127.0.0.1:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	scorer, _ := CreateAttentionScorer(AttentionScorerConfig{Client: c, Model: "jev"})
	//nolint:staticcheck
	_, err = scorer.Score(nil, AttentionIn{Subject: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func decodeSystemOneBody(r *http.Request) (map[string]any, error) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body, nil
}

func TestAttentionScorerNoDeadline(t *testing.T) {
	c, err := polypus.Create("http://127.0.0.1:1", nil)
	if err != nil {
		t.Fatal(err)
	}
	scorer, _ := CreateAttentionScorer(AttentionScorerConfig{Client: c, Model: "jev"})
	_, err = scorer.Score(context.Background(), AttentionIn{Subject: "x", About: "y"})
	if err == nil {
		t.Fatal("expected error")
	}
}
