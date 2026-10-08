package mailreport

import (
	"testing"

	"github.com/behaviorengineering/taxonomy/pkg/catalog"

	"github.com/xynova/should-i-read/internal/pimdir"
)

func testInboxCatalog(t *testing.T) *catalog.Catalog {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "inbox-mail",
		Terms: []catalog.Term{
			{ID: "unwanted", Label: "Unwanted", Description: "bulk"},
			{ID: "notification", Label: "Notification", Parent: "unwanted", Description: "alerts"},
			{ID: "newsletter", Label: "Newsletter", Parent: "unwanted", Description: "lists"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestHeuristicAssign_github(t *testing.T) {
	cat := testInboxCatalog(t)
	pre, ok := HeuristicAssign(pimdir.MailHeaders{From: "GitHub <notifications@github.com>"}, "", cat)
	if !ok || pre.TermID != "notification" || pre.Source != SourceHeuristic {
		t.Fatalf("heuristic: %+v ok=%v", pre, ok)
	}
}

func TestHeuristicAssign_listUnsubscribeOnly(t *testing.T) {
	cat := testInboxCatalog(t)
	sender := "SEEK Recommendations <noreply@s.seek.com.au>"
	_, ok := HeuristicAssign(pimdir.MailHeaders{
		From:            sender,
		ListUnsubscribe: "<https://www.seek.com.au/unsubscribe>",
	}, sender, cat)
	if ok {
		t.Fatal("List-Unsubscribe alone must not heuristic-assign newsletter")
	}
}

func TestHeuristicAssign_listIDNewsletter(t *testing.T) {
	cat := testInboxCatalog(t)
	pre, ok := HeuristicAssign(pimdir.MailHeaders{ListID: "<weekly.example.com>"}, "", cat)
	if !ok || pre.TermID != "newsletter" {
		t.Fatalf("List-Id: %+v ok=%v", pre, ok)
	}
}

func TestHeuristicAssign_bulkWithoutListID(t *testing.T) {
	cat := testInboxCatalog(t)
	pre, ok := HeuristicAssign(pimdir.MailHeaders{
		Precedence:      "bulk",
		ListUnsubscribe: "<mailto:unsub@example.com>",
	}, "", cat)
	if !ok || pre.TermID != "notification" {
		t.Fatalf("bulk without List-Id: %+v ok=%v", pre, ok)
	}
}

func TestSenderCatalogAssign_mapsTo(t *testing.T) {
	inbox := testInboxCatalog(t)
	senders, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "senders",
		Terms: []catalog.Term{{
			ID: "sender-github-notifications", Label: "GitHub", MapsTo: "notification",
			Patterns: []catalog.FieldPattern{{Field: "from", Re: "^.*notifications@github\\.com.*$"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"from": "notifications@github.com"}
	pre, ok := SenderCatalogAssign(senders, inbox, fields)
	if !ok || pre.TermID != "notification" || pre.Source != SourceSenderCatalog {
		t.Fatalf("sender catalog: %+v ok=%v", pre, ok)
	}
}

func TestShouldLearnSenders(t *testing.T) {
	if !ShouldLearnSenders("notification") || ShouldLearnSenders("spam") {
		t.Fatal("learn allow-list")
	}
}

func testSendersGitHubCatalog(t *testing.T) *catalog.Catalog {
	cat, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "senders",
		Terms: []catalog.Term{{
			ID: "sender-github-notifications", Label: "GitHub", MapsTo: "notification",
			Patterns: []catalog.FieldPattern{{Field: "from", Re: "^.*notifications@github\\.com.*$"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestMatchSenders_github(t *testing.T) {
	senders := testSendersGitHubCatalog(t)
	rt, ok := MatchSenders(senders, map[string]string{"from": "notifications@github.com"})
	if !ok || rt.ID != "sender-github-notifications" || rt.MapsTo != "notification" {
		t.Fatalf("match: %+v ok=%v", rt, ok)
	}
}

func TestMatchSenders_nilOrEmpty(t *testing.T) {
	senders := testSendersGitHubCatalog(t)
	_, ok := MatchSenders(nil, map[string]string{"from": "x"})
	if ok {
		t.Fatal("nil catalog")
	}
	_, ok = MatchSenders(senders, nil)
	if ok {
		t.Fatal("nil fields")
	}
}

func TestMatchSenders_spamMapsTo_stillMatches(t *testing.T) {
	inbox := testInboxCatalog(t)
	senders, err := catalog.BuildCatalog(catalog.Vocabulary{
		ID: "senders",
		Terms: []catalog.Term{{
			ID: "sender-spammy", Label: "Spammy", MapsTo: "spam",
			Patterns: []catalog.FieldPattern{{Field: "from", Re: "^spam@example\\.com$"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"from": "spam@example.com"}
	rt, ok := MatchSenders(senders, fields)
	if !ok || rt.ID != "sender-spammy" {
		t.Fatalf("match senders: %+v ok=%v", rt, ok)
	}
	_, assignOK := SenderCatalogAssign(senders, inbox, fields)
	if assignOK {
		t.Fatal("SenderCatalogAssign must reject maps_to spam")
	}
}
