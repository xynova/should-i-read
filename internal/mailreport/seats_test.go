package mailreport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/behaviorengineering/strop/pkg/jev"
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
