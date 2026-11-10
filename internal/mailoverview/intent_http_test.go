package mailoverview

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIntentPageGetAndGuess(t *testing.T) {
	t.Parallel()
	chat := &stubChat{out: "They are scanning the reply list."}
	g, err := CreateIntentGuesser(IntentConfig{Chat: chat, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	srv := &IntentPageServer{HTML: []byte("<html>overview</html>"), Guess: g}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}

	body, err := json.Marshal(intentRequest{Month: "Nov 2026", Text: "Need a reply 3"})
	if err != nil {
		t.Fatal(err)
	}
	pres, err := http.Post(ts.URL+"/intent", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer pres.Body.Close()
	if pres.StatusCode != http.StatusOK {
		t.Fatalf("intent status %d", pres.StatusCode)
	}
	var out intentResponse
	if err := json.NewDecoder(pres.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.OK || out.Intent != "They are scanning the reply list." {
		t.Fatalf("reply %+v", out)
	}
}

func TestIntentPageRejectsEmptySelection(t *testing.T) {
	t.Parallel()
	g, err := CreateIntentGuesser(IntentConfig{Chat: &stubChat{out: "x"}, Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer((&IntentPageServer{HTML: []byte("h"), Guess: g}).Handler())
	t.Cleanup(srv.Close)
	res, err := http.Post(srv.URL+"/intent", "application/json", strings.NewReader(`{"text":"  "}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status %d", res.StatusCode)
	}
}
