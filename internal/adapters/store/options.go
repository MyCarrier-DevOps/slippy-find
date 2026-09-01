package store

import (
	"context"
	"math/rand/v2"
	"time"
)

// Retry defaults. slippy-find resolves the correlation ID that every downstream step of a
// routing slip keys on, so a single dropped packet to slippy-api used to fail a production
// release outright. These values trade a few seconds of worst-case latency for that.
const (
	// defaultRetryAttempts is the total number of attempts, not the number of retries.
	defaultRetryAttempts = 4

	// defaultRetryBaseDelay seeds the exponential schedule. Equal jitter halves it, so the
	// first wait is uniform in [250ms, 500ms) — not 500ms.
	defaultRetryBaseDelay = 500 * time.Millisecond

	// maxRetryDelay caps the exponential backoff. A server-supplied Retry-After longer
	// than this is NOT trimmed to fit; the caller gives up instead. See delayFor.
	maxRetryDelay = 5 * time.Second
)

// Option customizes a SlipAPIAdapter.
type Option func(*adapterOptions)

// WithIPv4Only forces slippy-api dials onto IPv4. Set this on IPv4-only hosts to skip an
// AAAA leg that is wasted work there. It is an optimisation, not a correctness fix — see
// the README's "Network Resilience" section.
func WithIPv4Only(ipv4Only bool) Option {
	return func(o *adapterOptions) {
		o.ipv4Only = ipv4Only
	}
}

// WithRetryNotifier registers a callback invoked before each retry, so a caller can log
// that slippy-api needed another attempt. Retries that eventually succeed are otherwise
// invisible, which hides a degrading API until it fails outright.
func WithRetryNotifier(notify func(ctx context.Context, attempt int, delay time.Duration, err error)) Option {
	return func(o *adapterOptions) {
		if notify != nil {
			o.notifyRetry = notify
		}
	}
}

// retryPolicy describes how a failed slippy-api call is retried.
type retryPolicy struct {
	attempts  int
	baseDelay time.Duration
	maxDelay  time.Duration

	// budget bounds the whole retry sequence — every attempt plus every backoff.
	budget time.Duration

	// jitter returns a random duration in [0, d). Injectable so tests stay deterministic.
	jitter func(d time.Duration) time.Duration
}

// adapterOptions holds the caller-settable configuration for a SlipAPIAdapter.
type adapterOptions struct {
	ipv4Only    bool
	notifyRetry func(ctx context.Context, attempt int, delay time.Duration, err error)
}

func defaultAdapterOptions() adapterOptions {
	return adapterOptions{
		notifyRetry: func(context.Context, int, time.Duration, error) {},
	}
}

func defaultRetryPolicy() retryPolicy {
	return retryPolicy{
		attempts:  defaultRetryAttempts,
		baseDelay: defaultRetryBaseDelay,
		maxDelay:  maxRetryDelay,
		budget:    retryBudget,
		jitter:    randomJitter,
	}
}

// randomJitter returns a random duration in [0, d).
func randomJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(d)))
}

// sleepCtx waits for d, returning the context error if the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// delayFor returns how long to wait before the retry that follows attempt n (1-based).
//
// A server-supplied Retry-After is a floor, never a ceiling: it is honoured in full and
// then jittered UPWARD. Trimming it to maxDelay would retry earlier than the server asked,
// and slippy-api deliberately rounds Retry-After up so that a client arriving exactly at
// the boundary does not take another rung of its lockout ladder. Callers must refuse to
// retry at all when retryAfter exceeds the budget — see FindByCommits.
//
// Without a Retry-After the delay doubles per attempt and carries equal jitter — half
// fixed, half random — so concurrent CI jobs knocked off by the same blip do not
// resynchronise onto the same retry instant.
func (p retryPolicy) delayFor(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter + p.jitter(p.baseDelay)
	}

	delay := p.baseDelay
	for i := 1; i < attempt && delay < p.maxDelay; i++ {
		delay *= 2
	}
	delay = min(delay, p.maxDelay)

	fixed := delay / 2
	return fixed + p.jitter(delay-fixed)
}
