package pimalaya

import "testing"

func TestParseStreamEventScanProgressed(t *testing.T) {
	t.Parallel()
	ev, ok := ParseStreamEvent([]byte(`{"event":"scan-progressed","source":"imap","done":3,"total":9}`))
	if !ok {
		t.Fatal("expected event")
	}
	if ev.ProgressLine() != "Scanning imap 3/9" {
		t.Fatalf("line %q", ev.ProgressLine())
	}
}

func TestParseStreamEventSkipsAccountsReport(t *testing.T) {
	t.Parallel()
	if _, ok := ParseStreamEvent([]byte(`{"accounts":[]}`)); ok {
		t.Fatal("accounts report is not a progress event")
	}
}

func TestParseStreamEventListedAndOpened(t *testing.T) {
	t.Parallel()
	listed, ok := ParseStreamEvent([]byte(`{"event":"listed","endpoint":"imap","collections":11}`))
	if !ok || listed.ProgressLine() != "Listed 11 folders" {
		t.Fatalf("%+v ok=%v", listed, ok)
	}
	opened, ok := ParseStreamEvent([]byte(`{"event":"opened","endpoint":"imap"}`))
	if !ok || opened.ProgressLine() != "Opened imap" {
		t.Fatalf("%+v ok=%v", opened, ok)
	}
	started, ok := ParseStreamEvent([]byte(`{"event":"account-started","account":"gmail","position":1,"total":1}`))
	if !ok || started.ProgressLine() != "Syncing gmail" {
		t.Fatalf("%+v ok=%v", started, ok)
	}
}
