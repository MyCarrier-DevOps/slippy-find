package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingHandler wraps a handler and records how many requests reached it.
func countingHandler(calls *atomic.Int32, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		h(w, r)
	}
}

func writeSlip(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(slipAPIResponse{
		MatchedCommit: "abc123",
		Slip:          slipObj{CorrelationID: "test-correlation-id"},
	})
}

func TestFindByCommits_RetriesServerErrorThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	_, adapter, delays := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Load() < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeSlip(w)
	}))

	slip, matchedCommit, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.NoError(t, err)
	require.NotNil(t, slip)
	assert.Equal(t, "test-correlation-id", slip.CorrelationID)
	assert.Equal(t, "abc123", matchedCommit)
	assert.Equal(t, int32(3), calls.Load(), "should have retried twice before succeeding")
	assert.Len(t, *delays, 2, "one backoff per retry")
}

func TestFindByCommits_NeverRetriesSlippyAPILockout(t *testing.T) {
	// slippy-api's own 429 is an authentication-failure lockout, and every request that
	// arrives while locked charges another rung of a Fibonacci ladder capped at 7 days.
	// It is identified by X-RateLimit-Limit, which setRateLimitHeaders always sets when
	// the limiter state exists — and a 429 implies it does.
	var calls atomic.Int32
	_, adapter, _ := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "3")
		w.Header().Set("Retry-After", "20")
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load(), "a lockout 429 must cost exactly one request")
	assert.Contains(t, err.Error(), "extends the lockout")
	assert.Contains(t, err.Error(), "SLIPPY_API_KEY")
	assert.Contains(t, err.Error(), "wait 20s")
	assert.NotContains(t, err.Error(), "failed after")
}

func TestFindByCommits_LockoutMessageOmitsWaitWithoutRetryAfter(t *testing.T) {
	_, adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "3")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "wait for the lockout to clear")
	assert.NotContains(t, err.Error(), "wait 0s", "a missing Retry-After must not render as 0s")
}

func TestFindByCommits_RetriesIntermediaryThrottle(t *testing.T) {
	// A 429 without X-RateLimit-Limit did not come from slippy-api. Cloudflare fronts this
	// API and its 429 is an ordinary throttle — and today it is the only 429 that can
	// occur at all, since slippy-api ships with rate limiting disabled.
	var calls atomic.Int32
	_, adapter, _ := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Load() < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeSlip(w)
	}))

	slip, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.NoError(t, err)
	require.NotNil(t, slip)
	assert.Equal(t, int32(2), calls.Load())
}

func TestFindByCommits_RetriesTransportError(t *testing.T) {
	// A closed server produces a connection-refused transport error — the same class of
	// failure as the ENETUNREACH that broke production releases.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	adapter, err := NewSlipAPIAdapter(url, "test-key")
	require.NoError(t, err)

	var delays []time.Duration
	adapter.sleep = func(_ context.Context, d time.Duration) error {
		delays = append(delays, d)
		return nil
	}

	_, _, err = adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 4 attempts")
	assert.Len(t, delays, 3, "three backoffs across four attempts")
}

func TestFindByCommits_ExhaustsAttemptsOnPersistentServerError(t *testing.T) {
	var calls atomic.Int32
	_, adapter, _ := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed after 4 attempts")
	assert.Contains(t, err.Error(), "unexpected status 502")
	assert.Equal(t, int32(defaultRetryAttempts), calls.Load())
}

func TestFindByCommits_DoesNotRetryDefinitiveAnswers(t *testing.T) {
	cases := map[string]int{
		"not found":    http.StatusNotFound,
		"unauthorized": http.StatusUnauthorized,
		"forbidden":    http.StatusForbidden,
		"bad request":  http.StatusBadRequest,
	}

	for name, status := range cases {
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			_, adapter, _ := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))

			_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

			if status == http.StatusNotFound {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "failed after")
			}
			assert.Equal(t, int32(1), calls.Load(), "definitive answers must not be retried")
		})
	}
}

func TestFindByCommits_DoesNotRetryCancelledContext(t *testing.T) {
	var calls atomic.Int32
	_, adapter, _ := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		writeSlip(w)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := adapter.FindByCommits(ctx, "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "failed after", "caller cancellation is not a transient fault")
	assert.Equal(t, int32(0), calls.Load())
}

func TestFindByCommits_HonorsRetryAfterHeader(t *testing.T) {
	var calls atomic.Int32
	_, adapter, delays := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Load() < 2 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeSlip(w)
	}))

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.NoError(t, err)
	require.Len(t, *delays, 1)
	assert.GreaterOrEqual(t, (*delays)[0], 2*time.Second, "never retry earlier than the server asked")
	assert.Less(t, (*delays)[0], 2*time.Second+defaultRetryBaseDelay, "jittered upward, not by more than baseDelay")
}

func TestFindByCommits_HonorsRetryAfterLongerThanBackoffCap(t *testing.T) {
	// maxDelay shapes our own exponential backoff; it says nothing about what the sequence
	// can afford. A 503 + Retry-After: 10 during a rolling restart is recoverable inside
	// the budget, and must not be discarded for exceeding a backoff-shaping constant.
	var calls atomic.Int32
	_, adapter, delays := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Load() < 2 {
			w.Header().Set("Retry-After", "10")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeSlip(w)
	}))

	slip, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.NoError(t, err)
	require.NotNil(t, slip)
	require.Len(t, *delays, 1)
	assert.GreaterOrEqual(t, (*delays)[0], 10*time.Second, "honoured in full, never trimmed")
}

func TestFindByCommits_NotifiesEachRetry(t *testing.T) {
	type notice struct {
		attempt int
		delay   time.Duration
	}
	var notices []notice

	var calls atomic.Int32
	_, adapter, _ := newTestServer(t,
		countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
			if calls.Load() < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			writeSlip(w)
		}),
		WithRetryNotifier(func(_ context.Context, attempt int, delay time.Duration, err error) {
			require.Error(t, err)
			notices = append(notices, notice{attempt: attempt, delay: delay})
		}),
	)

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.NoError(t, err)
	require.Len(t, notices, 2)
	assert.Equal(t, 1, notices[0].attempt)
	assert.Equal(t, 2, notices[1].attempt)
	assert.Positive(t, notices[0].delay)
}

func TestWithRetryNotifier_NilIsIgnored(t *testing.T) {
	// A nil notifier must not replace the no-op default, or every retry would panic.
	_, adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}, WithRetryNotifier(nil))

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})
	require.Error(t, err)
}

func TestRetryPolicy_DelayFor(t *testing.T) {
	// No jitter, so the exponential progression is exactly assertable.
	policy := retryPolicy{
		attempts:  5,
		baseDelay: 500 * time.Millisecond,
		maxDelay:  5 * time.Second,
		jitter:    func(time.Duration) time.Duration { return 0 },
	}

	// Half of the exponential value, because the other half is the jitter budget.
	assert.Equal(t, 250*time.Millisecond, policy.delayFor(1, 0))
	assert.Equal(t, 500*time.Millisecond, policy.delayFor(2, 0))
	assert.Equal(t, time.Second, policy.delayFor(3, 0))
	assert.Equal(t, 2*time.Second, policy.delayFor(4, 0))

	assert.Equal(t, 2500*time.Millisecond, policy.delayFor(99, 0), "growth is capped at maxDelay")
	assert.Equal(t, time.Second, policy.delayFor(1, time.Second), "Retry-After wins over backoff")
	assert.Equal(t, time.Hour, policy.delayFor(1, time.Hour),
		"Retry-After is a floor, never trimmed — the caller refuses to retry instead")
}

func TestRetryPolicy_DelayForStaysWithinJitterBounds(t *testing.T) {
	policy := defaultRetryPolicy()

	for attempt := 1; attempt <= 10; attempt++ {
		delay := policy.delayFor(attempt, 0)
		assert.Positive(t, delay)
		assert.LessOrEqual(t, delay, policy.maxDelay)
	}
}

func TestRandomJitter(t *testing.T) {
	assert.Equal(t, time.Duration(0), randomJitter(0))
	assert.Equal(t, time.Duration(0), randomJitter(-time.Second))

	for range 100 {
		d := randomJitter(time.Second)
		assert.GreaterOrEqual(t, d, time.Duration(0))
		assert.Less(t, d, time.Second)
	}
}

func TestSleepCtx(t *testing.T) {
	require.NoError(t, sleepCtx(context.Background(), 0))
	require.NoError(t, sleepCtx(context.Background(), time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, sleepCtx(ctx, time.Hour), context.Canceled)
}

func TestRetryAfterDelay(t *testing.T) {
	assert.Equal(t, time.Duration(0), retryAfterDelay(nil))

	withHeader := func(value string) *http.Response {
		h := http.Header{}
		if value != "" {
			h.Set("Retry-After", value)
		}
		return &http.Response{Header: h}
	}

	assert.Equal(t, time.Duration(0), retryAfterDelay(withHeader("")))
	assert.Equal(t, 3*time.Second, retryAfterDelay(withHeader("3")))
	assert.Equal(t, time.Duration(0), retryAfterDelay(withHeader("0")))
	assert.Equal(t, time.Duration(0), retryAfterDelay(withHeader("-5")))
	assert.Equal(t, time.Duration(0), retryAfterDelay(withHeader("soon")))

	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	assert.Equal(t, time.Duration(0), retryAfterDelay(withHeader(past)))

	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	assert.Positive(t, retryAfterDelay(withHeader(future)))
}

func TestDialNetwork(t *testing.T) {
	assert.Equal(t, "tcp", dialNetwork("tcp", false))
	assert.Equal(t, "tcp4", dialNetwork("tcp", true))
	assert.Equal(t, "tcp6", dialNetwork("tcp6", true), "an explicit family is never rewritten")
	assert.Equal(t, "tcp4", dialNetwork("tcp4", false))
}

func TestNewHTTPClient(t *testing.T) {
	client, err := newHTTPClient(true)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, attemptTimeout, client.Timeout)
	require.NotNil(t, client.CheckRedirect, "redirects must be refused so the bearer token cannot be replayed")
	require.ErrorIs(t, client.CheckRedirect(nil, nil), http.ErrUseLastResponse,
		"a hook returning nil silently follows the redirect — Go only strips Authorization "+
			"when the host changes, so a same-host https->http redirect leaks the token")

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.DialContext, "the custom dialer must be installed")

	// The client still works end to end over loopback with IPv4 forced.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestWithIPv4Only(t *testing.T) {
	opts := defaultAdapterOptions()
	assert.False(t, opts.ipv4Only, "dual-stack is the default")

	WithIPv4Only(true)(&opts)
	assert.True(t, opts.ipv4Only)

	WithIPv4Only(false)(&opts)
	assert.False(t, opts.ipv4Only)
}

func TestNewSlipAPIAdapter_InstallsRetryDefaults(t *testing.T) {
	// Every other test substitutes adapter.sleep, so without this the constructor's
	// wiring is never asserted: a nil sleep would panic on the first production retry
	// and the whole suite would still be green.
	adapter, err := NewSlipAPIAdapter("http://slippy-api.invalid", "test-key")
	require.NoError(t, err)

	require.NotNil(t, adapter.sleep)
	require.NotNil(t, adapter.notifyRetry)
	require.NotNil(t, adapter.retry.jitter)
	assert.Equal(t, defaultRetryAttempts, adapter.retry.attempts)
	assert.Equal(t, defaultRetryBaseDelay, adapter.retry.baseDelay)
	assert.Equal(t, maxRetryDelay, adapter.retry.maxDelay)
	assert.Equal(t, retryBudget, adapter.retry.budget)

	// The wired no-op notifier must tolerate being called.
	assert.NotPanics(t, func() {
		adapter.notifyRetry(context.Background(), 1, time.Second, errors.New("boom"))
	})
}

func TestFindByCommits_RejectsInvalidRetryPolicy(t *testing.T) {
	// Guards the zero-value collision: attemptOutcome{} means "no slip found", so a loop
	// that never runs would otherwise return (nil, "", nil) — a definitive wrong answer
	// with no HTTP call made, which resolver.go renders as ErrNoAncestorSlip.
	var calls atomic.Int32
	_, adapter, _ := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		writeSlip(w)
	}))
	adapter.retry.attempts = 0

	slip, matchedCommit, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err, "must not masquerade as a successful no-slip-found answer")
	assert.Contains(t, err.Error(), "invalid retry policy")
	assert.Nil(t, slip)
	assert.Empty(t, matchedCommit)
	assert.Equal(t, int32(0), calls.Load())
}

func TestFindByCommits_DoesNotRetryUnparseableBody(t *testing.T) {
	// A 200 the client cannot parse means the request SUCCEEDED; the fault is
	// deterministic and reproduces byte for byte. Retrying costs four authenticated
	// POSTs and mislabels it "failed after 4 attempts".
	var calls atomic.Int32
	_, adapter, _ := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not json"))
	}))

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.Equal(t, int32(1), calls.Load(), "a parse failure must not be replayed")
	assert.NotContains(t, err.Error(), "failed after")
}

func TestFindByCommits_ReportsBudgetExhaustionFromBackoff(t *testing.T) {
	// Driven through the injected sleeper rather than a wall-clock budget: the budget
	// expires during a backoff far more often than during an attempt, and asserting it
	// deterministically beats racing a real timer on a loaded CI runner.
	var calls atomic.Int32
	_, adapter, _ := newTestServer(t, countingHandler(&calls, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	adapter.sleep = func(context.Context, time.Duration) error { return context.DeadlineExceeded }

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exhausted the 50s retry budget after 1 attempts")
	assert.Contains(t, err.Error(), "unexpected status 503", "the underlying fault stays visible")
	assert.Equal(t, int32(1), calls.Load(), "the budget must stop the sequence, not run it out")
}

func TestFindByCommits_BackoffAbortDistinguishesCallerFromBudget(t *testing.T) {
	// A non-deadline sleep error is the caller's, not the budget's, and must not be
	// reported as budget exhaustion.
	_, adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	adapter.sleep = func(context.Context, time.Duration) error { return context.Canceled }

	_, _, err := adapter.FindByCommits(context.Background(), "org/repo", []string{"abc123"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "aborted during retry backoff")
	assert.NotContains(t, err.Error(), "retry budget")
}

func TestIsRetryableRequestError(t *testing.T) {
	wrap := func(err error) error {
		return &url.Error{Op: "Post", URL: "https://slippy-api.example/slips/find-by-commits", Err: err}
	}

	// Terminal: slippy-api abandons its ancestry walk on disconnect and restarts from
	// commits[0], so a retry redoes the identical work and fails at the identical point.
	assert.False(t, isRetryableRequestError(wrap(context.DeadlineExceeded)),
		"a deadline means the server was mid-walk or already done — retrying is futile")
	assert.False(t, isRetryableRequestError(&timeoutErr{}),
		"http.Client.Timeout during the body read arrives unwrapped and is still a deadline")

	// Terminal: deterministic misconfiguration.
	assert.False(t, isRetryableRequestError(wrap(&net.DNSError{Err: "no such host", IsNotFound: true})))
	assert.False(t, isRetryableRequestError(wrap(&tls.CertificateVerificationError{})))
	assert.False(t, isRetryableRequestError(wrap(x509.HostnameError{Host: "slippy-api.example"})))

	// Retryable: the request never reached a server that did work on it.
	assert.True(t, isRetryableRequestError(wrap(syscall.ENETUNREACH)),
		"ENETUNREACH is the exact failure that motivated the retry")
	assert.True(t, isRetryableRequestError(wrap(syscall.ECONNREFUSED)))
	assert.True(t, isRetryableRequestError(wrap(&net.DNSError{Err: "server misbehaving", IsTemporary: true})),
		"a SERVFAIL is not an NXDOMAIN")

	// Retryable: a body truncated by a mid-read reset arrives bare, NOT as a *url.Error.
	// Gating on *url.Error would have classified this deterministic — it is not.
	assert.True(t, isRetryableRequestError(&net.OpError{Op: "read", Err: syscall.ECONNRESET}),
		"a mid-body reset is a transport fault even though it never passes through Do")
}

// timeoutErr stands in for *http.timeoutError, which is unexported. What matters to
// isRetryableRequestError is that it satisfies net.Error with Timeout() == true.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "context deadline exceeded (Client.Timeout ...)" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

func TestIsDecodeError(t *testing.T) {
	assert.True(t, isDecodeError(&json.SyntaxError{}))
	assert.True(t, isDecodeError(&json.UnmarshalTypeError{Value: "string", Type: nil}))
	assert.False(t, isDecodeError(&net.OpError{Op: "read", Err: syscall.ECONNRESET}),
		"a truncated body is a transport fault, not a decode fault")
}
