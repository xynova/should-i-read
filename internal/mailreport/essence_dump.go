package mailreport

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/xynova/should-i-read/internal/sirerr"
)

const essenceDumpIDUnknown = "unknown"

type essenceDumpIDCtxKey struct{}

// EssenceDumpConfig configures CreateEssenceDumper.
type EssenceDumpConfig struct {
	Dir string
	Now time.Time
}

// EssenceParseDump is the last failed essence chat completion for one message.
type EssenceParseDump struct {
	ObjectHash string `json:"object_hash,omitempty"`
	Model      string `json:"model,omitempty"`
	Slave      string `json:"slave,omitempty"`
	Attempt    int    `json:"attempt"`
	ParseError string `json:"parse_error,omitempty"`
	Message    string `json:"message,omitempty"`
	Raw        string `json:"raw"`
	WrittenAt  string `json:"written_at,omitempty"`
}

type essenceDumper struct {
	dir string
	now time.Time
}

// CreateEssenceDumper writes last-attempt essence parse failures under Dir.
// Empty Dir disables dumps (returns nil).
func CreateEssenceDumper(cfg EssenceDumpConfig) *essenceDumper {
	dir := strings.TrimSpace(cfg.Dir)
	if dir == "" {
		return nil
	}
	return &essenceDumper{dir: dir, now: cfg.Now}
}

func withEssenceDumpID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		return ctx
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, essenceDumpIDCtxKey{}, id)
}

func essenceDumpIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(essenceDumpIDCtxKey{}).(string)
	return strings.TrimSpace(id)
}

func (d *essenceDumper) writeLast(id, model, message, raw, slave string, attempt int, parseErr error) (string, error) {
	const op = "mailreport.essenceDumper.writeLast"
	if d == nil {
		return "", nil
	}
	if strings.TrimSpace(d.dir) == "" {
		return "", sirerr.New(sirerr.CodeInvalid, op, "empty dump dir")
	}
	if err := os.MkdirAll(d.dir, 0o750); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "mkdir essence dump")
	}
	id = sanitizeEssenceDumpID(id)
	rec := EssenceParseDump{
		ObjectHash: id,
		Model:      strings.TrimSpace(model),
		Slave:      strings.TrimSpace(slave),
		Attempt:    attempt,
		Message:    message,
		Raw:        raw,
	}
	if !d.now.IsZero() {
		rec.WrittenAt = d.now.UTC().Format(time.RFC3339)
	}
	if parseErr != nil {
		rec.ParseError = parseErr.Error()
	}
	rawJSON, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "encode essence dump")
	}
	path := filepath.Join(d.dir, id+".json")
	if err := os.WriteFile(path, append(rawJSON, '\n'), 0o600); err != nil {
		return "", sirerr.Wrap(err, sirerr.CodeFailed, op, "write essence dump")
	}
	return path, nil
}

func attachEssenceDump(err error, path string, dumpErr error) error {
	if err == nil {
		return nil
	}
	var typed *sirerr.Error
	if !errors.As(err, &typed) {
		typed = sirerr.Wrap(err, sirerr.CodeAI, "mailreport.attachEssenceDump", "essence dump")
	}
	if dumpErr != nil {
		return typed.With("dump_error", dumpErr.Error())
	}
	if strings.TrimSpace(path) != "" {
		return typed.With("dump", path)
	}
	return typed
}

func sanitizeEssenceDumpID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return essenceDumpIDUnknown
	}
	var b strings.Builder
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return essenceDumpIDUnknown
	}
	if len(out) > 80 {
		return out[:80]
	}
	return out
}
