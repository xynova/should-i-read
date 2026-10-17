package mailtags

import (
	"bufio"
	"os"
	"slices"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

// Config holds paths for the mail tag catalog.
type Config struct {
	RegisterPath     string
	DeprecationsPath string
	SeedPath         string
}

// Catalog holds active tag ids, deprecations, and seed ids for register rendering.
type Catalog struct {
	active   []string
	deprec   map[string]string
	depRows  []Deprecation
	seedIDs  []string
	register string
	seed     string
	depPath  string
}

// CreateCatalog loads register (or seed), deprecations, and seed list.
func CreateCatalog(cfg Config) (*Catalog, error) {
	seedIDs, err := LoadSeedIDs(cfg.SeedPath)
	if err != nil {
		return nil, err
	}
	deps, err := LoadDeprecations(cfg.DeprecationsPath)
	if err != nil {
		return nil, err
	}
	active, err := loadActiveIDs(cfg.RegisterPath, seedIDs)
	if err != nil {
		return nil, err
	}
	return &Catalog{
		active:   active,
		deprec:   deprecationMap(deps),
		depRows:  deps,
		seedIDs:  seedIDs,
		register: strings.TrimSpace(cfg.RegisterPath),
		seed:     strings.TrimSpace(cfg.SeedPath),
		depPath:  strings.TrimSpace(cfg.DeprecationsPath),
	}, nil
}

// ActiveIDs returns tag ids for the essence prompt prefer list.
func (c *Catalog) ActiveIDs() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.active)
}

// SeedIDs returns seed tag ids.
func (c *Catalog) SeedIDs() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.seedIDs)
}

// Deprecations returns deprecation rows for register rendering.
func (c *Catalog) Deprecations() []Deprecation {
	if c == nil {
		return nil
	}
	return slices.Clone(c.depRows)
}

// RegisterPath returns the operator register path from config.
func (c *Catalog) RegisterPath() string {
	if c == nil {
		return ""
	}
	return c.register
}

// NormalizeTokens applies deprecations, validates each token, and returns sorted unique ids.
func (c *Catalog) NormalizeTokens(tokens []string) ([]string, error) {
	const op = "mailtags.Catalog.NormalizeTokens"
	if len(tokens) == 0 {
		return nil, sirerr.New(sirerr.CodeAI, op, EssenceErrTagInvalid)
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		tok = normalizeToken(tok)
		if tok == "" {
			continue
		}
		if c != nil && c.deprec != nil {
			if to, ok := c.deprec[tok]; ok {
				tok = to
			}
		}
		if err := ValidateToken(tok); err != nil {
			return nil, err
		}
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	if len(out) == 0 {
		return nil, sirerr.New(sirerr.CodeAI, op, EssenceErrTagInvalid)
	}
	slices.Sort(out)
	return out, nil
}

// NormalizeField splits raw tag text, normalizes, and joins for classify_state.shape.
func (c *Catalog) NormalizeField(raw string) (string, error) {
	tokens := SplitTags(raw)
	ids, err := c.NormalizeTokens(tokens)
	if err != nil {
		return "", err
	}
	return JoinTags(ids), nil
}

// LoadSeedIDs reads one kebab id per line from the seed file.
func LoadSeedIDs(seedPath string) ([]string, error) {
	const op = "mailtags.LoadSeedIDs"
	seedPath = strings.TrimSpace(seedPath)
	if seedPath == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty seed path")
	}
	f, err := os.Open(seedPath)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "open seed").With("path", seedPath)
	}
	defer func() { _ = f.Close() }()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = normalizeToken(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "read seed")
	}
	if len(out) == 0 {
		return nil, sirerr.New(sirerr.CodeFailed, op, "seed file empty after trim").With("path", seedPath)
	}
	return out, nil
}

func loadActiveIDs(registerPath string, seedIDs []string) ([]string, error) {
	registerPath = strings.TrimSpace(registerPath)
	if registerPath != "" {
		ids, err := parseRegisterIDs(registerPath)
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}
	return slices.Clone(seedIDs), nil
}

func parseRegisterIDs(path string) ([]string, error) {
	const op = "mailtags.parseRegisterIDs"
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "read register").With("path", path)
	}
	var out []string
	inActive := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "## Active tags" {
			inActive = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inActive = false
			continue
		}
		if !inActive || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		tag, _, _ := strings.Cut(line, "\t")
		tag = normalizeToken(tag)
		if tag != "" {
			out = append(out, tag)
		}
	}
	slices.Sort(out)
	return out, nil
}
