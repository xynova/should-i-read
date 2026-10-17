package mailtags

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitTags_andJoin(t *testing.T) {
	got := SplitTags("marketing-pitch, promotional; time-critical\nfyi")
	want := []string{"marketing-pitch", "promotional", "time-critical", "fyi"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
	joined := JoinTags([]string{"b", "a", "b"})
	if joined != "a b" {
		t.Fatalf("join %q", joined)
	}
}

func TestValidateToken_rejectsGeometryAndPath(t *testing.T) {
	for _, tok := range []string{"rectangular", "a > b", "Bad Case"} {
		if err := ValidateToken(tok); err == nil {
			t.Fatalf("expected reject %q", tok)
		}
	}
	if err := ValidateToken("marketing-pitch"); err != nil {
		t.Fatal(err)
	}
}

func TestDeprecationRemap(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "mail-tag-deprecations.toml")
	if err := os.WriteFile(dep, []byte(`[[deprecated]]
from = "promo"
to = "promotional"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	deps, err := LoadDeprecations(dep)
	if err != nil || len(deps) != 1 {
		t.Fatalf("deps %v err %v", deps, err)
	}
	seed := filepath.Join(dir, "seed.txt")
	if err := os.WriteFile(seed, []byte("promotional\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cat, err := CreateCatalog(Config{SeedPath: seed, DeprecationsPath: dep})
	if err != nil {
		t.Fatal(err)
	}
	out, err := cat.NormalizeField("promo")
	if err != nil || out != "promotional" {
		t.Fatalf("got %q err %v", out, err)
	}
}

func TestLoadDeprecations_duplicateFrom(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "d.toml")
	body := `[[deprecated]]
from = "a"
to = "b"
[[deprecated]]
from = "a"
to = "c"
`
	if err := os.WriteFile(dep, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDeprecations(dep); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRenderRegister_golden(t *testing.T) {
	body := RenderRegister(map[string]int{"marketing-pitch": 3}, []string{"fyi"}, nil)
	if !strings.Contains(body, "marketing-pitch\t3") {
		t.Fatalf("%q", body)
	}
	if !strings.Contains(body, "fyi\t0") {
		t.Fatalf("seed at zero count: %q", body)
	}
}

func TestEnsureFiles_noOverwriteRegister(t *testing.T) {
	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.txt")
	reg := filepath.Join(dir, "register.txt")
	dep := filepath.Join(dir, "dep.toml")
	if err := os.WriteFile(seed, []byte("fyi\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(reg, []byte("custom\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureFiles(reg, dep, seed); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(reg)
	if err != nil || string(b) != "custom\n" {
		t.Fatalf("register overwritten: %q", b)
	}
}
