package mailreport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xynova/should-i-read/internal/pimdir"
	"github.com/xynova/should-i-read/internal/sirerr"
)

const (
	BenchFreezeMessageCount = 100
	BenchChunkSize          = 20
	BenchChunkCount         = 5
)

// BenchExportEnvelope matches mail export JSON consumed by itemsFromExportFile.
type BenchExportEnvelope struct {
	Mode           string                `json:"mode"`
	ExportedAt     string                `json:"exported_at"`
	StoreDir       string                `json:"store_dir"`
	PolypusBaseURL string                `json:"polypus_base_url"`
	Count          int                   `json:"count"`
	Emails         []pimdir.EmailSummary `json:"emails"`
}

// BenchFreezeResult records paths written by FreezeBenchInput.
type BenchFreezeResult struct {
	InputPath    string
	RevisionPath string
	ChunksDir    string
	ContentSHA   string
	Revision     string
}

// FreezeBenchInput reads an export file, dedupes and sorts emails, writes input-100 and chunk files.
func FreezeBenchInput(exportPath, outDir, gitSHA string) (BenchFreezeResult, error) {
	const op = "mailreport.FreezeBenchInput"
	exportPath = strings.TrimSpace(exportPath)
	outDir = strings.TrimSpace(outDir)
	if exportPath == "" || outDir == "" {
		return BenchFreezeResult{}, sirerr.New(sirerr.CodeInvalid, op, "export path and out dir required")
	}
	raw, err := os.ReadFile(exportPath)
	if err != nil {
		return BenchFreezeResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "read export")
	}
	var env BenchExportEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return BenchFreezeResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode export")
	}
	emails := dedupeExportEmails(env.Emails)
	if len(emails) < BenchFreezeMessageCount {
		return BenchFreezeResult{}, sirerr.New(sirerr.CodeInvalid, op, "export email count mismatch").
			With("want", strconv.Itoa(BenchFreezeMessageCount)).With("got", strconv.Itoa(len(emails)))
	}
	if len(emails) > BenchFreezeMessageCount {
		emails = emails[:BenchFreezeMessageCount]
	}
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return BenchFreezeResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir out dir")
	}
	chunksDir := filepath.Join(outDir, "chunks")
	if err := os.MkdirAll(chunksDir, 0o750); err != nil {
		return BenchFreezeResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir chunks")
	}
	frozen := BenchExportEnvelope{
		Mode:           env.Mode,
		ExportedAt:     env.ExportedAt,
		StoreDir:       env.StoreDir,
		PolypusBaseURL: env.PolypusBaseURL,
		Count:          len(emails),
		Emails:         emails,
	}
	inputPath := filepath.Join(outDir, "input-100.json")
	if err := writeBenchJSON(inputPath, frozen); err != nil {
		return BenchFreezeResult{}, err
	}
	contentSHA, err := fileSHA256(inputPath)
	if err != nil {
		return BenchFreezeResult{}, err
	}
	for i := 0; i < BenchChunkCount; i++ {
		start := i * BenchChunkSize
		end := start + BenchChunkSize
		chunk := BenchExportEnvelope{
			Mode:           frozen.Mode,
			ExportedAt:     frozen.ExportedAt,
			StoreDir:       frozen.StoreDir,
			PolypusBaseURL: frozen.PolypusBaseURL,
			Count:          BenchChunkSize,
			Emails:         emails[start:end],
		}
		chunkPath := filepath.Join(chunksDir, fmt.Sprintf("c%d.json", i+1))
		if err := writeBenchJSON(chunkPath, chunk); err != nil {
			return BenchFreezeResult{}, err
		}
	}
	rev := buildInputRevision(time.Now().UTC(), env.StoreDir, contentSHA, gitSHA)
	revPath := filepath.Join(outDir, "input-rev.txt")
	if err := os.WriteFile(revPath, []byte(rev+"\n"), 0o600); err != nil {
		return BenchFreezeResult{}, sirerr.Wrap(err, sirerr.CodeFailed, op, "write revision")
	}
	return BenchFreezeResult{
		InputPath:    inputPath,
		RevisionPath: revPath,
		ChunksDir:    chunksDir,
		ContentSHA:   contentSHA,
		Revision:     rev,
	}, nil
}

func dedupeExportEmails(in []pimdir.EmailSummary) []pimdir.EmailSummary {
	seen := map[string]struct{}{}
	out := make([]pimdir.EmailSummary, 0, len(in))
	for _, e := range in {
		h := strings.TrimSpace(e.ObjectHash)
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.TrimSpace(out[i].ObjectHash) < strings.TrimSpace(out[j].ObjectHash)
	})
	return out
}

func writeBenchJSON(path string, env BenchExportEnvelope) error {
	const op = "mailreport.writeBenchJSON"
	raw, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode json")
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write file").With("path", path)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	const op = "mailreport.fileSHA256"
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "read file")
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func buildInputRevision(now time.Time, storeDir, contentSHA, gitSHA string) string {
	return fmt.Sprintf("freeze_utc=%s store_dir=%s content_sha256=%s git_sha=%s",
		now.Format(time.RFC3339),
		strings.TrimSpace(storeDir),
		strings.TrimSpace(contentSHA),
		strings.TrimSpace(gitSHA),
	)
}

// ReadInputRevision loads the revision string from input-rev.txt.
func ReadInputRevision(path string) (string, error) {
	const op = "mailreport.ReadInputRevision"
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "read revision")
	}
	return strings.TrimSpace(string(raw)), nil
}

var benchChunkFileRe = regexp.MustCompile(`^c[0-9]+\.json$`)

// ListBenchChunkFiles returns sorted cN.json chunk paths in a chunks directory.
func ListBenchChunkFiles(chunksDir string) ([]string, error) {
	const op = "mailreport.ListBenchChunkFiles"
	entries, err := os.ReadDir(chunksDir)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "read chunks dir")
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if benchChunkFileRe.MatchString(name) {
			paths = append(paths, filepath.Join(chunksDir, name))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "no chunk files found").With("dir", chunksDir)
	}
	return paths, nil
}
