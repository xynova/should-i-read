package mailreport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/polypus"
)

func TestPolypusJudge_Decide_noulPerOption(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "cf_local/typesafe/jev",
			"answers": map[string]any{
				"use__notification": map[string]any{"type": "noul", "noul": 0.81},
				"skip":              map[string]any{"type": "noul", "noul": 0.2},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(client, "cf_local/typesafe/jev", "cf_local/gemma")
	if err != nil {
		t.Fatal(err)
	}
	out, err := seats.Judge.Decide(context.Background(), harness.DecideIn{
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
				"use__notification": map[string]any{"type": "noul", "noul": 0.5},
				"skip":              map[string]any{"type": "noul", "noul": 0.5},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(client, "jev", "chat")
	if err != nil {
		t.Fatal(err)
	}
	out, err := seats.Judge.Decide(context.Background(), harness.DecideIn{
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
				"use__notification": map[string]any{"type": "noul", "noul": 0.7},
				"use__newsletter":   map[string]any{"type": "noul", "noul": 0.7},
				"skip":              map[string]any{"type": "noul", "noul": 0.1},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(client, "jev", "chat")
	if err != nil {
		t.Fatal(err)
	}
	out, err := seats.Judge.Decide(context.Background(), harness.DecideIn{
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
				harness.ChoiceAcceptDraft: map[string]any{"type": "noul", "noul": 0.8},
				harness.ChoiceReject:      map[string]any{"type": "noul", "noul": 0.8},
			},
		})
	}))
	t.Cleanup(srv.Close)
	client, err := polypus.Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	seats, err := CreateSeats(client, "jev", "chat")
	if err != nil {
		t.Fatal(err)
	}
	out, err := seats.Judge.Decide(context.Background(), harness.DecideIn{
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

func TestCreateSeats_emptyJudge(t *testing.T) {
	c, err := polypus.Create("http://127.0.0.1:1320", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CreateSeats(c, "", "chat"); err == nil {
		t.Fatal("expected error")
	}
}
