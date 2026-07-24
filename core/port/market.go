package port

import (
	"context"

	"danmo-work/core/domain"
)

// Market is a content source for the expert/skill marketplace.
type Market interface {
	SourceID() string
	Kind() string

	// FetchCatalog downloads and parses the source catalog index.
	FetchCatalog(ctx context.Context) (domain.MarketCatalog, error)

	// FetchPackage materializes an item into a local directory.
	// Caller must invoke cleanup when done (may be a no-op).
	FetchPackage(ctx context.Context, item domain.MarketItem, ref string) (localDir string, cleanup func(), err error)
}

// MarketRegistry holds configured Market adapters and can be rebuilt from config.
type MarketRegistry interface {
	List() []Market
	Get(sourceID string) (Market, bool)
	Reload(sources []domain.MarketSource) error
}
