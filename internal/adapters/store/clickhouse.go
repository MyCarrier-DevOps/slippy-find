// Package store provides adapters for slip storage backends.
package store

import (
	"context"

	"github.com/MyCarrier-DevOps/goLibMyCarrier/slippy"

	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
)

// ClickHouseAdapter wraps goLibMyCarrier's SlipStore to implement domain.SlipFinder.
// This adapter translates between the external library types and our domain types.
type ClickHouseAdapter struct {
	store slippy.SlipStore
}

// NewClickHouseAdapter creates a new adapter wrapping the given SlipStore.
func NewClickHouseAdapter(store slippy.SlipStore) *ClickHouseAdapter {
	return &ClickHouseAdapter{
		store: store,
	}
}

// FindByCommits searches for a slip matching any of the given commits.
// Returns the slip, the matched commit SHA, and any error.
// Returns (nil, "", nil) if no matching slip is found.
func (a *ClickHouseAdapter) FindByCommits(
	ctx context.Context,
	repository string,
	commits []string,
) (*domain.Slip, string, error) {
	slip, matchedCommit, err := a.store.FindByCommits(ctx, repository, commits)
	if err != nil {
		return nil, "", err
	}

	if slip == nil {
		return nil, "", nil
	}

	// Convert to domain type
	return &domain.Slip{
		CorrelationID: slip.CorrelationID,
	}, matchedCommit, nil
}

// Close releases any resources held by the store.
func (a *ClickHouseAdapter) Close() error {
	return a.store.Close()
}
