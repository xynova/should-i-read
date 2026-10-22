package mailreport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/behaviorengineering/strop/pkg/jev"
	"github.com/behaviorengineering/taxonomy/pkg/catalog"
	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/polypus"
)

func judgeKey(choice string) string {
	return jev.JoinKey(classifyRowID, choice)
}

func TestPolypusJudge_Decide_noulPerOption(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "cf_local/typesafe/jev",
			"answers": map[string]any{
				judgeKey("use:notification"): map[string]any{"type": "noul", "noul": 0.81},
				judgeKey("skip"):             map[string]any{"type": "noul", "noul": 0.2},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(SeatsConfig{Client: client, JudgeModel: "cf_local/typesafe/jev", AuthorModel: "cf_local/gemma"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := seats.Judge.Decide(ctx, harness.DecideIn{
		Text: "Password reset for GitHub",
		Options: []harness.PackedOption{
			{Choice: "use:notification", Label: "Notification", Description: "alerts"},
			{Choice: "skip", Label: "Skip"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Choice != "use:notification" {
		t.Fatalf("choice: %q", out.Choice)
	}
	if out.Score != 0.81 {
		t.Fatalf("score: %v", out.Score)
	}
}

func TestPolypusJudge_Decide_skipOnTie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				judgeKey("use:notification"): map[string]any{"type": "noul", "noul": 0.5},
				judgeKey("skip"):             map[string]any{"type": "noul", "noul": 0.5},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(SeatsConfig{Client: client, JudgeModel: "jev", AuthorModel: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := seats.Judge.Decide(ctx, harness.DecideIn{
		Text: "x",
		Options: []harness.PackedOption{
			{Choice: "use:notification", Label: "Notification"},
			{Choice: "skip", Label: "Skip"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Choice != harness.ChoiceSkip {
		t.Fatalf("choice: %q", out.Choice)
	}
}

func TestPolypusJudge_Decide_skipOnUseSiblingTie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				judgeKey("use:notification"): map[string]any{"type": "noul", "noul": 0.7},
				judgeKey("use:newsletter"):   map[string]any{"type": "noul", "noul": 0.7},
				judgeKey("skip"):             map[string]any{"type": "noul", "noul": 0.1},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(SeatsConfig{Client: client, JudgeModel: "jev", AuthorModel: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := seats.Judge.Decide(ctx, harness.DecideIn{
		Text: "x",
		Options: []harness.PackedOption{
			{Choice: "use:notification", Label: "Notification"},
			{Choice: "use:newsletter", Label: "Newsletter"},
			{Choice: "skip", Label: "Skip"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Choice != harness.ChoiceSkip {
		t.Fatalf("choice: %q", out.Choice)
	}
}

func TestPolypusJudge_Decide_gateRejectOnTie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				judgeKey(harness.ChoiceAcceptDraft): map[string]any{"type": "noul", "noul": 0.8},
				judgeKey(harness.ChoiceReject):      map[string]any{"type": "noul", "noul": 0.8},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(SeatsConfig{Client: client, JudgeModel: "jev", AuthorModel: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := seats.Judge.Decide(ctx, harness.DecideIn{
		Text: "x",
		Options: []harness.PackedOption{
			{Choice: harness.ChoiceAcceptDraft, Label: "Accept"},
			{Choice: harness.ChoiceReject, Label: "Reject"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Choice != harness.ChoiceReject {
		t.Fatalf("choice: %q", out.Choice)
	}
}

func TestCreateSeats_sameOptionSetOneEval(t *testing.T) {
	var posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		posts.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers": map[string]any{
				jev.JoinKey("hash-a", "skip"):             map[string]any{"type": "noul", "noul": 0.1},
				jev.JoinKey("hash-a", "use:notification"): map[string]any{"type": "noul", "noul": 0.9},
				jev.JoinKey("hash-b", "skip"):             map[string]any{"type": "noul", "noul": 0.2},
				jev.JoinKey("hash-b", "use:notification"): map[string]any{"type": "noul", "noul": 0.8},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(SeatsConfig{Client: client, JudgeModel: "jev", AuthorModel: "chat"})
	if err != nil {
		t.Fatal(err)
	}
	judge := seats.Judge.(*polypusJudge)
	opts := []harness.PackedOption{
		{Choice: "use:notification", Label: "Notification"},
		{Choice: "skip", Label: "Skip"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	outs, err := judge.BatchDecide(ctx, []JudgeBatchItem{
		{RowID: "hash-a", In: harness.DecideIn{Text: "a", Options: opts}},
		{RowID: "hash-b", In: harness.DecideIn{Text: "b", Options: opts}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 1 {
		t.Fatalf("eval posts=%d", posts.Load())
	}
	if len(outs) != 2 || outs[0].Choice != "use:notification" || outs[1].Choice != "use:notification" {
		t.Fatalf("outs=%v", outs)
	}
}

func TestCreateSeats_emptyJudge(t *testing.T) {
	c, err := polypus.Create("http://127.0.0.1:1320", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateSeats(SeatsConfig{Client: c, AuthorModel: "chat"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestPolypusEssencerSuccessOnAttempt2(t *testing.T) {
	whyOnly := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\n"
	full := whyOnly + "ABOUT: about\nTAGS: fyi\nKIND: notification > software-release > changelog\n"
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		n := calls.Add(1)
		content := whyOnly
		if n >= 2 {
			content = full
			var body struct {
				Messages []polypus.ChatMessage `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil && len(body.Messages) >= 4 {
				if body.Messages[2].Role != polypus.RoleAssistant || !strings.Contains(body.Messages[2].Content, "WHY1:") {
					t.Errorf("assistant turn: %+v", body.Messages[2])
				}
				if body.Messages[3].Role != polypus.RoleUser || !strings.Contains(strings.ToLower(body.Messages[3].Content), "continue") {
					t.Errorf("complete user turn: %+v", body.Messages[3])
				}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(SeatsConfig{
		Client:       client,
		JudgeModel:   "jev",
		AuthorModel:  "chat",
		EssenceModel: "granite",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := seats.Essencer.Essence(ctx, harness.EssenceIn{WorldContext: WorldContextAttach, Text: "Subject: t\n\nbody"})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.ParseKindSegments(out.Kind)) < 3 {
		t.Fatalf("kind %q", out.Kind)
	}
	if calls.Load() != 2 {
		t.Fatalf("chat calls=%d", calls.Load())
	}
}

func TestPolypusEssencerCompensateAfterMicroExhaust(t *testing.T) {
	whyOnly := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\n"
	full := whyOnly + "ABOUT: about\nTAGS: fyi\nKIND: notification > software-release > changelog\n"
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		calls.Add(1)
		var body struct {
			Model    string                `json:"model"`
			Messages []polypus.ChatMessage `json:"messages"`
		}
		content := whyOnly
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Model == "granite-8b" {
			if len(body.Messages) != 2 {
				t.Errorf("escalate restart expected 2 messages, got %d: %+v", len(body.Messages), body.Messages)
			}
			content = full
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(SeatsConfig{
		Client:               client,
		JudgeModel:           "jev",
		AuthorModel:          "chat",
		EssenceModel:         "granite",
		EssenceEscalateModel: "granite-8b",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := seats.Essencer.Essence(ctx, harness.EssenceIn{WorldContext: WorldContextAttach, Text: "Subject: t\n\nbody"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Kind == "" {
		t.Fatal("expected kind")
	}
	if calls.Load() != 4 {
		t.Fatalf("chat calls=%d", calls.Load())
	}
}

func TestPolypusEssencerWhy4RestartDoesNotContinueTranscript(t *testing.T) {
	why4First := "WHY4: delivered promptly\nWHY5: short layout\nTAGS: job-alert\nKIND: notification > job-alert > software-engineering\n"
	full := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nABOUT: about\nTAGS: fyi\nKIND: notification > software-release > changelog\n"
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		n := calls.Add(1)
		var body struct {
			Messages []polypus.ChatMessage `json:"messages"`
		}
		content := why4First
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && n == 2 {
			if len(body.Messages) != 2 {
				t.Errorf("second granite chat expected 2 messages after restart, got %d", len(body.Messages))
			}
			content = full
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(SeatsConfig{
		Client:       client,
		JudgeModel:   "jev",
		AuthorModel:  "chat",
		EssenceModel: "granite",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := seats.Essencer.Essence(ctx, harness.EssenceIn{WorldContext: WorldContextAttach, Text: "Subject: t\n\nbody"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Kind == "" {
		t.Fatal("expected kind")
	}
	if calls.Load() != 2 {
		t.Fatalf("chat calls=%d", calls.Load())
	}
}

func TestPolypusEssencerDumpsLastParseFailure(t *testing.T) {
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nKIND: gitlab > pipeline\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": raw}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	seats, err := CreateSeats(SeatsConfig{
		Client:         client,
		JudgeModel:     "jev",
		AuthorModel:    "chat",
		EssenceModel:   "granite",
		EssenceDumpDir: dir,
		Now:            time.Date(2026, 10, 13, 15, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = withEssenceDumpID(ctx, "qmdiovp3ibpm6np7uzh4z3dg52vn5b7l3ia54ezijbyhcjyn3x7a")
	_, err = seats.Essencer.Essence(ctx, harness.EssenceIn{WorldContext: WorldContextAttach, Text: "Subject: consilium | Failed pipeline\n\nbody"})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "dump=") {
		t.Fatalf("missing dump field: %v", err)
	}
	path := filepath.Join(dir, "qmdiovp3ibpm6np7uzh4z3dg52vn5b7l3ia54ezijbyhcjyn3x7a.json")
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(got), `"raw": "WHY1: a`) {
		t.Fatalf("dump missing raw: %s", got)
	}
	if !strings.Contains(string(got), "about or shape empty") {
		t.Fatalf("dump missing parse_error: %s", got)
	}
}

func TestPolypusEssencerDumpsAfterEscalateStillInvalid(t *testing.T) {
	raw := "WHY1: a\nWHY2: b\nWHY3: c\nWHY4: d\nWHY5: e\nKIND: gitlab > pipeline\n"
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": raw}},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	seats, err := CreateSeats(SeatsConfig{
		Client:               client,
		JudgeModel:           "jev",
		AuthorModel:          "chat",
		EssenceModel:         "granite",
		EssenceEscalateModel: "granite-8b",
		EssenceDumpDir:       dir,
		Now:                  time.Date(2026, 10, 13, 15, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = withEssenceDumpID(ctx, "escalate-dump-id")
	_, err = seats.Essencer.Essence(ctx, harness.EssenceIn{WorldContext: WorldContextAttach, Text: "Subject: t\n\nbody"})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if calls.Load() != 6 {
		t.Fatalf("chat calls=%d", calls.Load())
	}
	path := filepath.Join(dir, "escalate-dump-id.json")
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	body := string(got)
	if !strings.Contains(body, `"model": "granite-8b"`) {
		t.Fatalf("dump model: %s", body)
	}
	if !strings.Contains(body, `"attempt": 6`) {
		t.Fatalf("dump attempt: %s", body)
	}
}
