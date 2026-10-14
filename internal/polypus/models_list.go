package polypus

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xynova/should-i-read/internal/sirerr"
)

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type apiCatalogResponse struct {
	APIs []apiCatalogEntry `json:"apis"`
}

type apiCatalogEntry struct {
	ID     string `json:"id"`
	Models string `json:"models"`
}

// listEnabledModelIDs returns merged model ids from GET /v1/apis flavor lists.
func (c *Client) listEnabledModelIDs(ctx context.Context) ([]string, error) {
	const op = "polypus.Client.listEnabledModelIDs"

	catalogBody, err := c.get(ctx, "/v1/apis")
	if err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeUnavailable, op, "polypus /v1/apis unreachable").
			With("base_url", c.BaseURL)
	}
	ids, err := modelIDsFromAPICatalog(ctx, c, catalogBody)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, sirerr.New(sirerr.CodeUnavailable, op, "no enabled models; check Polypus allow-lists").
			With("base_url", c.BaseURL)
	}
	return ids, nil
}

func modelIDsFromAPICatalog(ctx context.Context, c *Client, catalogBody json.RawMessage) ([]string, error) {
	const op = "polypus.modelIDsFromAPICatalog"
	var cat apiCatalogResponse
	if err := json.Unmarshal(catalogBody, &cat); err != nil {
		return nil, sirerr.Wrap(err, sirerr.CodeFailed, op, "decode /v1/apis")
	}
	seen := map[string]struct{}{}
	var ids []string
	for _, entry := range cat.APIs {
		path := strings.TrimSpace(entry.Models)
		if path == "" || !strings.HasPrefix(path, "/") {
			continue
		}
		body, err := c.get(ctx, path)
		if err != nil {
			continue
		}
		flavorIDs, err := parseModelListBody(body)
		if err != nil {
			continue
		}
		for _, id := range flavorIDs {
			if id == "" {
				continue
			}
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, sirerr.New(sirerr.CodeUnavailable, op, "no models from /v1/apis flavor lists").
			With("base_url", c.BaseURL)
	}
	return ids, nil
}

func parseModelListBody(body json.RawMessage) ([]string, error) {
	var models modelsResponse
	if err := json.Unmarshal(body, &models); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(models.Data))
	for _, m := range models.Data {
		if m.ID != "" {
			ids = append(ids, m.ID)
		}
	}
	return ids, nil
}
