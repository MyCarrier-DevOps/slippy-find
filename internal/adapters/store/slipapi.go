// Package store provides adapters for slip storage backends.
package store

import (
	"context"
	"fmt"
	"net/http"

	slippyclient "github.com/MyCarrier-DevOps/slippy-api/slippy-client"

	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
)

// SlipAPIAdapter implements domain.SlipFinder by calling the slippy-api HTTP service.
type SlipAPIAdapter struct {
	client *slippyclient.WrappedClient
}

// NewSlipAPIAdapter creates a SlipAPIAdapter targeting the given server URL with a bearer token.
func NewSlipAPIAdapter(serverURL, apiKey string) (*SlipAPIAdapter, error) {
	c, err := slippyclient.NewWrappedClient(
		serverURL,
		slippyclient.WithBearerToken(apiKey),
		slippyclient.WithServiceName("slippy-find"),
	)
	if err != nil {
		return nil, fmt.Errorf("slippy-api client: %w", err)
	}
	return &SlipAPIAdapter{client: c}, nil
}

// FindByCommits calls POST /v1/slips/find-by-commits and maps the result to domain types.
// Returns (nil, "", nil) if no matching slip is found (404).
func (a *SlipAPIAdapter) FindByCommits(
	ctx context.Context,
	repository string,
	commits []string,
) (*domain.Slip, string, error) {
	resp, err := a.client.FindByCommitsWithResponse(ctx, slippyclient.FindByCommitsJSONRequestBody{
		Repository: repository,
		Commits:    &commits,
	})
	if err != nil {
		return nil, "", fmt.Errorf("find-by-commits request: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusOK:
		if resp.JSON200 == nil {
			return nil, "", fmt.Errorf("find-by-commits: empty 200 response body")
		}
		return &domain.Slip{CorrelationID: resp.JSON200.Slip.CorrelationId}, resp.JSON200.MatchedCommit, nil
	case http.StatusNotFound:
		return nil, "", nil
	default:
		return nil, "", fmt.Errorf("find-by-commits: unexpected status %d", resp.StatusCode())
	}
}

// Close is a no-op; HTTP connections are managed by the transport.
func (a *SlipAPIAdapter) Close() error {
	return nil
}
