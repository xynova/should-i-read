package mailreport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFreezeBenchInput_deterministicChunks(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "export.json")
	writeSyntheticExport(exportPath, 100)

	res1, err := FreezeBenchInput(exportPath, filepath.Join(dir, "a"), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	res2, err := FreezeBenchInput(exportPath, filepath.Join(dir, "b"), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if res1.ContentSHA != res2.ContentSHA {
		t.Fatalf("sha mismatch %s vs %s", res1.ContentSHA, res2.ContentSHA)
	}
	chunks1, _ := ListBenchChunkFiles(res1.ChunksDir)
	chunks2, _ := ListBenchChunkFiles(res2.ChunksDir)
	for i := range chunks1 {
		h1, _ := fileSHA256(chunks1[i])
		h2, _ := fileSHA256(chunks2[i])
		if h1 != h2 {
			t.Fatalf("chunk %d hash mismatch", i)
		}
	}
}

func TestListBenchChunkFiles_ignoresCaseIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "c1.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "case-index.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err := ListBenchChunkFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "c1.json" {
		t.Fatalf("paths %v", paths)
	}
}

func TestFreezeBenchInput_chunkContract(t *testing.T) {
	dir := t.TempDir()
	exportPath := filepath.Join(dir, "export.json")
	writeSyntheticExport(exportPath, 100)
	out := filepath.Join(dir, "bench")
	res, err := FreezeBenchInput(exportPath, out, "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := ListBenchChunkFiles(res.ChunksDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != BenchChunkCount {
		t.Fatalf("chunks %d", len(chunks))
	}
	seen := map[string]struct{}{}
	for _, chunkPath := range chunks {
		var env BenchExportEnvelope
		raw, err := os.ReadFile(chunkPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Emails) != BenchChunkSize {
			t.Fatalf("%s rows %d", chunkPath, len(env.Emails))
		}
		for _, e := range env.Emails {
			h := strings.TrimSpace(e.ObjectHash)
			if h == "" {
				t.Fatal("empty hash in chunk")
			}
			if _, ok := seen[h]; ok {
				t.Fatalf("duplicate hash %s", h)
			}
			seen[h] = struct{}{}
		}
	}
	if len(seen) != BenchFreezeMessageCount {
		t.Fatalf("union size %d", len(seen))
	}
	var frozen BenchExportEnvelope
	raw, err := os.ReadFile(res.InputPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &frozen); err != nil {
		t.Fatal(err)
	}
	if len(frozen.Emails) != BenchFreezeMessageCount {
		t.Fatalf("frozen %d", len(frozen.Emails))
	}
}

func writeSyntheticExport(path string, n int) {
	emails := make([]map[string]string, 0, n+5)
	for i := 0; i < n; i++ {
		emails = append(emails, map[string]string{
			"object_hash": fmt.Sprintf("%064x", i),
			"subject":     fmt.Sprintf("subject-%d", i),
		})
	}
	// Extra rows exercise dedupe: empty hash and duplicate.
	emails = append(emails, map[string]string{"object_hash": "", "subject": "skip"})
	emails = append(emails, map[string]string{"object_hash": fmt.Sprintf("%064x", 0), "subject": "dup"})
	env := map[string]any{
		"mode":      "export",
		"store_dir": "/tmp/pim",
		"count":     len(emails),
		"emails":    emails,
	}
	raw, _ := json.MarshalIndent(env, "", "  ")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		panic(err)
	}
}
