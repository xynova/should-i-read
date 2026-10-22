package mailreport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strings"
	"sync"

	"github.com/behaviorengineering/taxonomy/pkg/harness"

	"github.com/xynova/should-i-read/internal/hoststore"
	"github.com/xynova/should-i-read/internal/sirerr"
)

// CachingEmbedderConfig configures a memoizing harness.Embedder.
type CachingEmbedderConfig struct {
	Inner harness.Embedder
	Model string
	Store *hoststore.Store
}

// CreateCachingEmbedder wraps inner with process-local and optional SQLite text cache.
func CreateCachingEmbedder(cfg CachingEmbedderConfig) (harness.Embedder, error) {
	const op = "mailreport.CreateCachingEmbedder"
	if cfg.Inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil inner embedder")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "empty embed model")
	}
	return &cachingEmbedder{
		inner: cfg.Inner,
		model: model,
		store: cfg.Store,
		mem:   map[string][]float64{},
	}, nil
}

type cachingEmbedder struct {
	inner harness.Embedder
	model string
	store *hoststore.Store
	mu    sync.Mutex
	mem   map[string][]float64
}

func (c *cachingEmbedder) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	const op = "mailreport.cachingEmbedder.Embed"
	if c == nil || c.inner == nil {
		return nil, sirerr.New(sirerr.CodeInvalid, op, "nil embedder")
	}
	if err := ctx.Err(); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "embed canceled")
	}
	out := make([][]float64, len(texts))
	type miss struct {
		text string
		sha  string
	}
	var misses []miss
	seenMiss := map[string]int{}
	for i, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, sirerr.New(sirerr.CodeInvalid, op, "empty embed text")
		}
		key := cacheTextKey(c.model, text)
		if vec, ok := c.lookupMem(key); ok {
			out[i] = copyVec(vec)
			continue
		}
		sha := textSHA256(text)
		if c.store != nil {
			vec, err := c.store.GetEmbedText(ctx, sha, c.model)
			if err != nil {
				return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "get embed text cache")
			}
			if vec != nil {
				c.storeMem(key, vec)
				out[i] = copyVec(vec)
				continue
			}
		}
		if _, ok := seenMiss[text]; !ok {
			seenMiss[text] = len(misses)
			misses = append(misses, miss{text: text, sha: sha})
		}
	}
	if len(misses) == 0 {
		return out, nil
	}
	uniqueTexts := make([]string, len(misses))
	for i, m := range misses {
		uniqueTexts[i] = m.text
	}
	vecs, err := c.inner.Embed(ctx, uniqueTexts)
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeAI, op, "inner embed")
	}
	if len(vecs) != len(uniqueTexts) {
		return nil, sirerr.New(sirerr.CodeFailed, op, "inner embed length mismatch")
	}
	for i, vec := range vecs {
		if err := validateEmbedVec(vec); err != nil {
			return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "invalid embed vector")
		}
		vec = copyVec(vec)
		c.storeMem(cacheTextKey(c.model, misses[i].text), vec)
		if c.store != nil {
			if err := c.store.PutEmbedText(ctx, misses[i].sha, c.model, vec); err != nil {
				return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "put embed text cache")
			}
		}
	}
	for i, text := range texts {
		if out[i] != nil {
			continue
		}
		text = strings.TrimSpace(text)
		key := cacheTextKey(c.model, text)
		if vec, ok := c.lookupMem(key); ok {
			out[i] = copyVec(vec)
			continue
		}
		return nil, sirerr.New(sirerr.CodeFailed, op, "cache fill incomplete")
	}
	return out, nil
}

func (c *cachingEmbedder) lookupMem(key string) ([]float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	vec, ok := c.mem[key]
	return vec, ok
}

func (c *cachingEmbedder) storeMem(key string, vec []float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mem[key] = copyVec(vec)
}

func cacheTextKey(model, text string) string {
	return model + "\x00" + text
}

func textSHA256(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func copyVec(vec []float64) []float64 {
	return append([]float64(nil), vec...)
}

func validateEmbedVec(vec []float64) error {
	const op = "mailreport.validateEmbedVec"
	if len(vec) == 0 {
		return sirerr.New(sirerr.CodeFailed, op, "empty vector")
	}
	for _, v := range vec {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return sirerr.New(sirerr.CodeFailed, op, "non-finite vector value")
		}
	}
	return nil
}
