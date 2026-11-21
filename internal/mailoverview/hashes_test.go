package mailoverview

import "testing"

func TestJoinHashes_firstSeenUnique(t *testing.T) {
	got := JoinHashes([]string{" a ", "a", "", "b", "a"})
	if got != "a,b" {
		t.Fatalf("got %q", got)
	}
	if JoinHashes(nil) != "" {
		t.Fatal("empty input should join to empty")
	}
}
