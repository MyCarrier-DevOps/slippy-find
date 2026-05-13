// Package store provides adapters for slip storage backends.
package store

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	slippyclient "github.com/MyCarrier-DevOps/slippy-api/slippy-client"

	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
)

// httpClientTimeout bounds slippy-api HTTP calls so a misconfigured URL or unreachable host
// fails fast instead of blocking the CLI until the workflow step is killed.
const httpClientTimeout = 30 * time.Second

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
		slippyclient.WithCustomHTTPClient(&http.Client{Timeout: httpClientTimeout}),
		// Discard the wrapper's per-request slog output; this binary's stderr contract is
		// "warnings/errors only" via the zap logger wired in main.
		slippyclient.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
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
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, "", fmt.Errorf(
			"find-by-commits: authentication failed (status %d) — check SLIPPY_API_KEY: %s",
			resp.StatusCode(), problemDetail(resp.ApplicationproblemJSONDefault),
		)
	default:
		return nil, "", fmt.Errorf(
			"find-by-commits: unexpected status %d: %s",
			resp.StatusCode(), problemDetail(resp.ApplicationproblemJSONDefault),
		)
	}
}

// problemDetail returns the RFC 7807 problem detail when available, otherwise an empty marker.
// This surfaces the slippy-api error body in the wrapped error message so operators can
// distinguish auth failures, validation errors, and server errors at a glance.
func problemDetail(p *slippyclient.ErrorModel) string {
	if p == nil {
		return "<no error body>"
	}
	if p.Detail != nil && *p.Detail != "" {
		return *p.Detail
	}
	if p.Title != nil && *p.Title != "" {
		return *p.Title
	}
	return "<no detail>"
}

// Close is a no-op; HTTP connections are managed by the transport.
func (a *SlipAPIAdapter) Close() error {
	return nil
}
