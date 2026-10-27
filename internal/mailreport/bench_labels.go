package mailreport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const benchLabelSubsetCount = 20

// WriteBenchLabelStub writes labels-20.json with subject hints for human review of expected_leaf and required_terms.
func WriteBenchLabelStub(input100Path, revision, outPath string) error {
	const op = "mailreport.WriteBenchLabelStub"
	raw, err := os.ReadFile(input100Path)
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "read input")
	}
	var env BenchExportEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "decode input")
	}
	if len(env.Emails) < benchLabelSubsetCount {
		return sirerr.New(sirerr.CodeInvalid, op, "input too small for label stub").
			With("want", strconv.Itoa(benchLabelSubsetCount)).With("got", strconv.Itoa(len(env.Emails)))
	}
	step := len(env.Emails) / benchLabelSubsetCount
	if step < 1 {
		step = 1
	}
	labels := make([]BenchLabel, 0, benchLabelSubsetCount)
	for i := 0; i < benchLabelSubsetCount; i++ {
		idx := i * step
		if idx >= len(env.Emails) {
			idx = len(env.Emails) - 1
		}
		e := env.Emails[idx]
		labels = append(labels, BenchLabel{
			ObjectHash:    strings.TrimSpace(e.ObjectHash),
			SubjectHint:   strings.TrimSpace(e.Subject),
			ExpectedLeaf:  "TODO",
			RequiredTerms: []string{"todo"},
		})
	}
	file := BenchLabelsFile{Revision: revision, Labels: labels}
	outPath = strings.TrimSpace(outPath)
	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(input100Path), "labels-20.json")
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "encode labels")
	}
	if err := os.WriteFile(outPath, append(encoded, '\n'), 0o600); err != nil {
		return sirerr.Wrap(err, sirerr.CodeFailed, op, "write labels").With("path", outPath)
	}
	return nil
}
