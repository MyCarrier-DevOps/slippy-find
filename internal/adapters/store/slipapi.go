// Package store provides adapters for slip storage backends.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	slippyclient "github.com/MyCarrier-DevOps/slippy-api/slippy-client"

	"github.com/MyCarrier-DevOps/slippy-find/internal/domain"
)

// SlipAPIAdapter implements domain.SlipFinder by calling the slippy-api HTTP service.
type SlipAPIAdapter struct {
	client      *slippyclient.WrappedClient
	retry       retryPolicy
	sleep       func(ctx context.Context, d time.Duration) error
	notifyRetry func(ctx context.Context, attempt int, delay time.Duration, err error)
}

// NewSlipAPIAdapter creates a SlipAPIAdapter targeting the given server URL with a bearer token.
func NewSlipAPIAdapter(serverURL, apiKey string, opts ...Option) (*SlipAPIAdapter, error) {
	options := defaultAdapterOptions()
	for _, opt := range opts {
		opt(&options)
	}

	httpClient, err := newHTTPClient(options.ipv4Only)
	if err != nil {
		return nil, fmt.Errorf("slippy-api client: %w", err)
	}

	c, err := slippyclient.NewWrappedClient(
		serverURL,
		slippyclient.WithBearerToken(apiKey),
		slippyclient.WithServiceName("slippy-find"),
		slippyclient.WithCustomHTTPClient(httpClient),
		// Discard the wrapper's per-request slog output; this binary's stderr contract is
		// "warnings/errors only" via the zap logger wired in main.
		slippyclient.WithLogger(slog.New(slog.DiscardHandler)),
	)
	if err != nil {
		return nil, fmt.Errorf("slippy-api client: %w", err)
	}

	return &SlipAPIAdapter{
		client:      c,
		retry:       defaultRetryPolicy(),
		sleep:       sleepCtx,
		notifyRetry: options.notifyRetry,
	}, nil
}

// FindByCommits calls POST /slips/find-by-commits and maps the result to domain types.
// Returns (nil, "", nil) if no matching slip is found (404).
//
// Retried: failures where the request never reached a server that then did work on it
// (dial faults), 5xx, and an intermediary 429. Not retried: 200, 404, 401/403, any other
// 4xx, a slippy-api 429, and — whatever the layer — a deadline or a body this client
// cannot decode. See isRetryableRequestError for why a deadline is terminal.
func (a *SlipAPIAdapter) FindByCommits(
	ctx context.Context,
	repository string,
	commits []string,
) (*domain.Slip, string, error) {
	if a.retry.attempts < 1 {
		return nil, "", fmt.Errorf("find-by-commits: invalid retry policy: attempts=%d", a.retry.attempts)
	}

	// Keep the caller's context distinct from the budget-bounded one. Retryability is
	// judged against the caller's — otherwise budget exhaustion would masquerade as the
	// caller cancelling, and findByCommitsOnce's "cancellation is the caller's decision"
	// rule would silently become false.
	callerCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, a.retry.budget)
	defer cancel()

	var last attemptOutcome

	for attempt := 1; attempt <= a.retry.attempts; attempt++ {
		last = a.findByCommitsOnce(ctx, callerCtx, repository, commits)
		if last.err == nil {
			return last.slip, last.matchedCommit, nil
		}
		// Checked before the retryable/last-attempt break and keyed on the error rather
		// than the classification, so budget exhaustion still reports accurately when it
		// lands on the final attempt or on an outcome we would not have retried.
		if a.budgetExpired(ctx, callerCtx, last.err) {
			return nil, "", a.budgetError(attempt, last.err)
		}
		if !last.retryable || attempt == a.retry.attempts {
			break
		}

		// No give-up on a long Retry-After: the budget context already bounds the wait,
		// and sleepCtx surfaces its expiry. Comparing against maxDelay would have thrown
		// away a retry the budget could comfortably afford — maxDelay shapes our own
		// backoff and says nothing about what the sequence can spend.
		delay := a.retry.delayFor(attempt, last.retryAfter)
		a.notifyRetry(callerCtx, attempt, delay, last.err)

		if err := a.sleep(ctx, delay); err != nil {
			if callerCtx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
				return nil, "", a.budgetError(attempt, last.err)
			}
			return nil, "", fmt.Errorf(
				"find-by-commits aborted during retry backoff: %w (last attempt: %w)", err, last.err,
			)
		}
	}

	if last.retryable {
		return nil, "", fmt.Errorf("find-by-commits failed after %d attempts: %w", a.retry.attempts, last.err)
	}
	return nil, "", last.err
}

// Close is a no-op; HTTP connections are managed by the transport.
func (a *SlipAPIAdapter) Close() error {
	return nil
}

// findByCommitsOnce performs a single find-by-commits call and classifies its outcome.
// reqCtx carries the retry budget; callerCtx is the caller's own, and only its
// cancellation makes a transport failure non-transient.
func (a *SlipAPIAdapter) findByCommitsOnce(
	reqCtx, callerCtx context.Context,
	repository string,
	commits []string,
) attemptOutcome {
	// The call and the parse are split so retryability can be decided from WHERE the
	// failure happened, not by sniffing error types. The generated client's combined
	// helper reads the body in ParseFindByCommitsResponse, after Do has returned, so a
	// body-read fault arrives unwrapped and is indistinguishable from a decode fault.
	// Splitting keeps auth and tracing intact — WrappedClient overrides no methods, it
	// installs RequestEditorFns on the client underneath.
	//nolint:bodyclose // ParseFindByCommitsResponse defers Close on every return path
	// (slippy-client client.gen.go:3510); bodyclose cannot see across that call.
	httpResp, err := a.client.FindByCommits(reqCtx, slippyclient.FindByCommitsJSONRequestBody{
		Repository: repository,
		Commits:    &commits,
	})
	if err != nil {
		// A cancelled or expired caller context is the caller's decision, not a fault.
		return attemptOutcome{
			err:       fmt.Errorf("find-by-commits request: %w", err),
			retryable: callerCtx.Err() == nil && isRetryableRequestError(err),
		}
	}
	resp, err := slippyclient.ParseFindByCommitsResponse(httpResp)
	if err != nil {
		// Headers arrived, so slippy-api accepted the request and did the work. A body
		// this client cannot decode will decode identically next time; a truncated or
		// reset body is a genuine transport fault and is still worth another attempt.
		return attemptOutcome{
			err: fmt.Errorf("find-by-commits response (status %d): %w", httpResp.StatusCode, err),
			retryable: callerCtx.Err() == nil &&
				!isDecodeError(err) && isRetryableRequestError(err),
		}
	}

	code := resp.StatusCode()
	switch code {
	case http.StatusOK:
		if resp.JSON200 == nil {
			return attemptOutcome{err: errors.New("find-by-commits: empty 200 response body")}
		}
		return attemptOutcome{
			slip:          &domain.Slip{CorrelationID: resp.JSON200.Slip.CorrelationId},
			matchedCommit: resp.JSON200.MatchedCommit,
		}

	case http.StatusNotFound:
		return attemptOutcome{}

	case http.StatusUnauthorized, http.StatusForbidden:
		return attemptOutcome{err: fmt.Errorf(
			"find-by-commits: authentication failed (status %d) — check SLIPPY_API_KEY: %s",
			code, problemDetail(resp.ApplicationproblemJSONDefault),
		)}

	case http.StatusTooManyRequests:
		// slippy-api's own 429 is an authentication-failure lockout, and it charges
		// another rung of a Fibonacci ladder for every request that arrives while locked
		// — so retrying escalates a per-client-IP lockout capped at seven days. It always
		// carries X-RateLimit-Limit: setRateLimitHeaders returns early unless the limiter
		// state exists, and a 429 implies it does.
		//
		// A 429 WITHOUT that header did not come from slippy-api. This API is fronted by
		// Cloudflare, whose 429 is an ordinary throttle and a legitimate retry candidate
		// — and today it is the only 429 that can actually occur, since slippy-api ships
		// with SLIPPY_RATE_LIMIT_ENABLED=false and nothing enables it.
		// Header.Get canonicalises the key; slippy-api writes it as "X-RateLimit-Limit".
		if resp.HTTPResponse != nil && resp.HTTPResponse.Header.Get("X-Ratelimit-Limit") != "" {
			return attemptOutcome{err: fmt.Errorf(
				"find-by-commits: rate limited (status %d) — slippy-api locks out repeated "+
					"authentication failures and each further request extends the lockout; "+
					"check SLIPPY_API_KEY%s: %s",
				code, lockoutWait(retryAfterDelay(resp.HTTPResponse)),
				problemDetail(resp.ApplicationproblemJSONDefault),
			)}
		}
		return attemptOutcome{
			err: fmt.Errorf(
				"find-by-commits: throttled upstream of slippy-api (status %d): %s",
				code, problemDetail(resp.ApplicationproblemJSONDefault),
			),
			retryable:  true,
			retryAfter: retryAfterDelay(resp.HTTPResponse),
		}
	}

	// Redirects are refused by the client, so a 3xx arrives here as a response rather
	// than being followed. The overwhelmingly likely cause is an http:// SLIPPY_API_URL
	// meeting an edge that upgrades to https, which is worth naming outright.
	if code >= http.StatusMultipleChoices && code < http.StatusBadRequest {
		return attemptOutcome{err: fmt.Errorf(
			"find-by-commits: unexpected redirect (status %d) to %q — redirects are refused so "+
				"the bearer token cannot be replayed; check that SLIPPY_API_URL uses https",
			code, httpResp.Header.Get("Location"),
		)}
	}

	// Everything else is an unexpected status. Server-side faults are worth another
	// attempt; the remaining 4xx are the caller's own request being wrong.
	return attemptOutcome{
		err: fmt.Errorf(
			"find-by-commits: unexpected status %d: %s",
			code, problemDetail(resp.ApplicationproblemJSONDefault),
		),
		retryable:  code >= http.StatusInternalServerError,
		retryAfter: retryAfterDelay(resp.HTTPResponse),
	}
}

// budgetExpired reports whether the retry budget — and not the caller's own context —
// is what ended this attempt.
func (a *SlipAPIAdapter) budgetExpired(reqCtx, callerCtx context.Context, err error) bool {
	return reqCtx.Err() != nil && callerCtx.Err() == nil && errors.Is(err, context.DeadlineExceeded)
}

func (a *SlipAPIAdapter) budgetError(attempt int, cause error) error {
	return fmt.Errorf(
		"find-by-commits: exhausted the %s retry budget after %d attempts: %w",
		a.retry.budget, attempt, cause,
	)
}

// lockoutWait renders the wait clause of the lockout message, omitting it when slippy-api
// sent no usable Retry-After rather than claiming a nonsensical "wait 0s".
func lockoutWait(d time.Duration) string {
	if d <= 0 {
		return " and wait for the lockout to clear"
	}
	return fmt.Sprintf(" and wait %s", d)
}

// attemptOutcome is the classified result of one find-by-commits call.
type attemptOutcome struct {
	slip          *domain.Slip
	matchedCommit string
	err           error

	// retryable marks err as a transient fault worth another attempt.
	retryable bool

	// retryAfter carries a server-supplied Retry-After, or 0 when none was sent.
	retryAfter time.Duration
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
