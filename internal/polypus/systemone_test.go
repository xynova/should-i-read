package polypus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemOne_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "cf_local/typesafe/jev",
			"answers": map[string]any{
				"leaf":  map[string]any{"type": "choice", "choice": "use:notification"},
				"score": map[string]any{"type": "noul", "noul": 0.9},
			},
		})
	}))
	defer srv.Close()
	c, err := Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.SystemOne(context.Background(), SystemOneRequest{
		Model: "cf_local/typesafe/jev",
		State: "a message",
		Questions: map[string]SystemOneQuestion{
			"leaf":  {Type: "choice", Instructions: "pick"},
			"score": {Type: "noul", Instructions: "confidence"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Answers["leaf"].ChoiceString() != "use:notification" {
		t.Fatalf("choice: %+v", out.Answers["leaf"])
	}
	if out.Answers["score"].Noul != 0.9 {
		t.Fatalf("noul: %+v", out.Answers["score"])
	}
}

func TestSystemOne_nilContext(t *testing.T) {
	c, err := Create("http://127.0.0.1:1320", nil)
	if err != nil {
		t.Fatal(err)
	}
	//nolint:staticcheck // nil context is the case under test
	_, err = c.SystemOne(nil, SystemOneRequest{Model: "m", State: "s", Questions: map[string]SystemOneQuestion{"q": {Type: "noul"}}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSystemOne_noulAnswers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "cf_local/typesafe/jev",
			"answers": map[string]any{
				"use:notification": map[string]any{"type": "noul", "noul": 0.9},
				"skip":             map[string]any{"type": "noul", "noul": 0.1},
			},
		})
	}))
	defer srv.Close()
	c, err := Create(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.SystemOne(context.Background(), SystemOneRequest{
		Model: "cf_local/typesafe/jev",
		State: "a message",
		Questions: map[string]SystemOneQuestion{
			"use:notification": {Type: "noul", Instructions: "fit"},
			"skip":             {Type: "noul", Instructions: "skip"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Answers["use:notification"].Noul != 0.9 {
		t.Fatalf("noul: %+v", out.Answers)
	}
}

func TestSystemOne_emptyModel(t *testing.T) {
	c, err := Create("http://127.0.0.1:1320", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SystemOne(context.Background(), SystemOneRequest{State: "s", Questions: map[string]SystemOneQuestion{"q": {Type: "noul"}}})
	if err == nil {
		t.Fatal("expected error")
	}
}
