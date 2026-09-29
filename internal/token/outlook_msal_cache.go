package token

import (
	"context"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/cache"
)

// keyringMSALCache implements MSAL ExportReplace against host keyring blobs.
type keyringMSALCache struct {
	account string
}

func (k *keyringMSALCache) Replace(ctx context.Context, c cache.Unmarshaler, _ cache.ReplaceHints) error {
	_ = ctx
	data, err := LoadOutlookMSALCache(k.account)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return c.Unmarshal(data)
}

func (k *keyringMSALCache) Export(ctx context.Context, c cache.Marshaler, _ cache.ExportHints) error {
	_ = ctx
	data, err := c.Marshal()
	if err != nil {
		return err
	}
	return SaveOutlookMSALCache(k.account, data)
}
