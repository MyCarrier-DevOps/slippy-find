package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Timeouts for slippy-api HTTP calls. A whole GitHub Actions step blocks on this call, so
// the budget that matters is the one bounding the entire retry sequence, not one attempt.
const (
	// attemptTimeout bounds ONE attempt end to end (connect, TLS, and the response-body
	// read). It is sized from slippy-api's own envelope, not from retryBudget/attempts:
	// on a ClickHouse miss, find-by-commits falls back to a serial per-commit GitHub
	// GraphQL ancestry walk, and slippy-api sets WriteTimeout: 60s with the comment
	// "60s leaves ample room for a capped ancestry walk". A client cap below that band
	// cannot serve a legitimately slow walk on ANY attempt, because every attempt gets
	// the same cap and the server restarts the walk from scratch each time.
	attemptTimeout = 45 * time.Second

	// retryBudget is the hard ceiling on the whole sequence — every attempt plus every
	// backoff. attempts x attemptTimeout is deliberately NOT reachable: a deadline is
	// classified non-retryable (see isRetryableRequestError), so the sequence can only
	// run long by accumulating fast failures, which cost dialTimeout each.
	retryBudget = 50 * time.Second

	// dialTimeout bounds one DialContext: name resolution plus every address tried, which
	// net.Dialer subdivides across the remaining addresses (with a 2s floor per address).
	// It is not a per-TCP-connect cap. This is what bounds the retryable failure class.
	dialTimeout = 5 * time.Second

	// keepAlive matches net/http.DefaultTransport's dialer.
	keepAlive = 30 * time.Second

	// maxRetryAfterSeconds clamps a server-supplied Retry-After before it is converted to
	// a Duration. time.Duration(seconds)*time.Second silently wraps past ~292 years, and
	// a crafted "Retry-After: 18446744074" wraps to 290ms.
	maxRetryAfterSeconds = int(24 * time.Hour / time.Second)
)

// newHTTPClient builds the HTTP client used for slippy-api calls.
//
// When ipv4Only is set, "tcp" dials are forced to "tcp4", skipping an AAAA leg that is
// wasted work on an IPv4-only host. See the README's "Network Resilience" section for why
// this is an optimisation and not the fix for a transient dial failure.
func newHTTPClient(ipv4Only bool) (*http.Client, error) {
	// Commit to the assertion rather than falling back to a zero-value Transport: that
	// fallback would silently drop Proxy (ignoring HTTPS_PROXY on proxied runners), lose
	// TLSHandshakeTimeout, and — combined with the custom DialContext below — disable
	// HTTP/2. Failing loudly at construction beats degrading invisibly at runtime.
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("http.DefaultTransport is %T, want *http.Transport", http.DefaultTransport)
	}
	transport := base.Clone()

	dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, dialNetwork(network, ipv4Only), addr)
	}

	return &http.Client{
		Timeout:   attemptTimeout,
		Transport: transport,
		// Refuse redirects. Go only strips Authorization when the redirect changes the
		// host, so an https -> http redirect to the SAME host keeps the bearer token and
		// puts SLIPPY_API_KEY on the wire in cleartext. slippy-api's find-by-commits
		// endpoint has no legitimate redirect, so any 3xx surfaces as an unexpected
		// status — which is non-retryable, and therefore fails fast and visibly.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}, nil
}

// dialNetwork narrows a dual-stack "tcp" dial to "tcp4" when IPv4 is forced. Networks that
// already name an address family are passed through untouched.
func dialNetwork(network string, ipv4Only bool) string {
	if ipv4Only && network == "tcp" {
		return "tcp4"
	}
	return network
}

// isRetryableRequestError reports whether a network-level failure is worth another attempt.
//
// The decisive question is not the error's type but whether slippy-api did work on the
// request. A dial that never reached it is free to repeat. A deadline is not: slippy-api's
// ancestry walk aborts on client disconnect and restarts from commits[0], so a retry redoes
// the identical walk, fails at the identical point, and burns the shared GitHub GraphQL
// quota four times over. That is why attemptTimeout can afford to be generous.
func isRetryableRequestError(err error) bool {
	// Covers both a context deadline and http.Client.Timeout, whether the deadline fired
	// waiting for headers (server still walking) or during the body read (server done).
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}

	// A hostname that does not resolve will not start resolving on attempt two. Note this
	// checks IsNotFound specifically — a SERVFAIL or a timed-out query stays retryable.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return false
	}

	// A certificate that fails verification fails identically every time.
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return false
	}
	var hostErr x509.HostnameError
	return !errors.As(err, &hostErr)
}

// isDecodeError reports whether err is the response body failing to unmarshal — the one
// post-header failure that is deterministic, since the bytes will decode identically next
// time. A truncated or reset body is a different thing and stays retryable.
func isDecodeError(err error) bool {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	return errors.As(err, &syntaxErr) || errors.As(err, &typeErr)
}

// retryAfterDelay reads a Retry-After header in either the delay-seconds or the HTTP-date
// form. It returns 0 when the header is absent, unparseable, or already in the past, which
// leaves the caller on its own exponential backoff.
func retryAfterDelay(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	value := resp.Header.Get("Retry-After")
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}
		// Clamp before converting, or the multiply wraps and a huge value reads as a
		// sub-second delay — retrying far earlier than the server asked.
		return time.Duration(min(seconds, maxRetryAfterSeconds)) * time.Second
	}

	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}

	return 0
}
