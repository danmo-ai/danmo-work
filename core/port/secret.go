package port

import "context"

// SecretStore holds sensitive values (API keys, OAuth tokens) outside prompts.
type SecretStore interface {
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	DeletePrefix(ctx context.Context, prefix string) error
}
