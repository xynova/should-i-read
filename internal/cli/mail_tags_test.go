package cli

import (
	"strings"
	"testing"
)

func TestFormatMailTags_includesPathsAndWrapsDropped(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := formatMailTags(mailTagsReport{
		RegisterPath:     "/cfg/vocabularies/mail-tag-register.txt",
		DeprecationsPath: "/cfg/vocabularies/mail-tag-deprecations.toml",
		SeedPath:         "/repo/config/vocabularies/mail-tags-seed.txt",
		StorePath:        "/data/classify.db",
		Rewrote:          22,
		Dropped:          []string{"article", "bullet-point", "business"},
		Merged:           map[string]string{"promo": "promotional", "note": "fyi"},
		ActiveTagIDs:     40,
	})
	want := []string{
		"Mail tags",
		"Register      /cfg/vocabularies/mail-tag-register.txt",
		"Deprecations  /cfg/vocabularies/mail-tag-deprecations.toml",
		"Seed          /repo/config/vocabularies/mail-tags-seed.txt",
		"Ledger        /data/classify.db",
		"Rewrote 22 classify row shape(s)",
		"40 active tag ids",
		"Dropped (3)",
		"article, bullet-point, business",
		"Merged (2)",
		"note -> fyi",
		"promo -> promotional",
	}
	for _, part := range want {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q in:\n%s", part, out)
		}
	}
}

func TestFormatMailTags_emptyLedger(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	out := formatMailTags(mailTagsReport{
		RegisterPath: "/cfg/register.txt",
		JudgeSkipped: true,
	})
	if !strings.Contains(out, "No tagged complete rows yet") {
		t.Fatalf("missing empty hint:\n%s", out)
	}
	if !strings.Contains(out, "Judge skipped") {
		t.Fatalf("missing judge skip:\n%s", out)
	}
}
