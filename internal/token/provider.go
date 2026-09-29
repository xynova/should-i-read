package token

import "context"

// Provider is a mailbox OAuth broker (Gmail and Outlook).
type Provider interface {
	Login(ctx context.Context) error
	AccessToken(ctx context.Context) (string, error)
	Logout(ctx context.Context) error
	Status() map[string]string
}
