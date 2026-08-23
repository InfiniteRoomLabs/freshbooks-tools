package freshbooks

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// An Option configures a Client during NewClient.
type Option func(*Client) error

// WithTokenSource supplies the access token for every request. Use
// auth.NewTokenSource for a source that refreshes and persists rotation, or
// auth.StaticTokenSource when the process was handed a token directly.
func WithTokenSource(ts auth.TokenSource) Option {
	return func(c *Client) error {
		if ts == nil {
			return fmt.Errorf("freshbooks: WithTokenSource needs a non-nil token source")
		}
		c.tokenSource = ts
		return nil
	}
}

// WithHTTPClient replaces the underlying HTTP client. The client is shallow
// copied, so the caller's value is never mutated; if it has no CheckRedirect
// the copy gets one that drops the bearer token on a cross-host redirect.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) error {
		if hc == nil {
			return fmt.Errorf("freshbooks: WithHTTPClient needs a non-nil client")
		}
		c.httpClient = hc
		return nil
	}
}

// WithBaseURL points the client at a different API root -- a fixture server
// in tests, or a sandbox. The URL must be absolute.
func WithBaseURL(raw string) Option {
	return func(c *Client) error {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("freshbooks: WithBaseURL(%q): %w", raw, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("freshbooks: WithBaseURL(%q): need an absolute URL", raw)
		}
		u.Path = strings.TrimSuffix(u.Path, "/")
		c.baseURL = u
		return nil
	}
}

// WithUserAgent sets the User-Agent header. Callers building a product on
// this library should identify it here.
func WithUserAgent(ua string) Option {
	return func(c *Client) error {
		if ua == "" {
			return fmt.Errorf("freshbooks: WithUserAgent needs a non-empty string")
		}
		c.userAgent = ua
		return nil
	}
}

// WithLogger attaches a logger. The client logs request method, URL, status,
// and attempt number at debug level only; it never logs headers, request
// bodies, or response bodies, so a token cannot reach the log through it.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) error {
		if l == nil {
			return fmt.Errorf("freshbooks: WithLogger needs a non-nil logger")
		}
		c.logger = l
		return nil
	}
}

// WithRetry replaces the retry policy. Pass NoRetry to disable retries.
func WithRetry(p RetryPolicy) Option {
	return func(c *Client) error {
		if p.MaxAttempts < 1 {
			return fmt.Errorf("freshbooks: WithRetry needs MaxAttempts >= 1, got %d", p.MaxAttempts)
		}
		c.retry = p
		return nil
	}
}

// WithClock replaces the clock the client reads for backoff and Retry-After
// arithmetic. Tests use it to make both deterministic.
func WithClock(now func() time.Time) Option {
	return func(c *Client) error {
		if now == nil {
			return fmt.Errorf("freshbooks: WithClock needs a non-nil function")
		}
		c.now = now
		return nil
	}
}

// RetryPolicy controls how the transport retries a request that failed with
// a transient status (429, 502, 503, 504) or a transport error.
//
// # At-least-once semantics
//
// Retrying is not free of consequences for writes. A 502, a 504, or a
// network failure can all arrive after the server has already processed the
// request, and the retry replays the body -- so a POST that creates an
// invoice or a payment can create two. Every method this library exposes is
// therefore at-least-once, not exactly-once, whenever retrying is enabled.
//
// Until the library gates retries by idempotency, a caller making a write it
// cannot afford to duplicate should pass WithRetry(NoRetry) for that client
// and handle the transient statuses itself.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts, not the number of
	// retries. 1 disables retrying.
	MaxAttempts int
	// BaseDelay is the first backoff interval; each further attempt
	// doubles it.
	BaseDelay time.Duration
	// MaxDelay caps a single wait, including one the server asked for via
	// Retry-After. A client should not block for an unbounded period just
	// because a header said so.
	MaxDelay time.Duration
	// Jitter randomizes a computed delay. Nil means full jitter: a
	// uniformly random duration in [0, d). Tests set it to the identity to
	// make waits deterministic.
	Jitter func(time.Duration) time.Duration
}

// NoRetry is the policy that makes every request a single attempt.
var NoRetry = RetryPolicy{MaxAttempts: 1}

// DefaultRetryPolicy is what NewClient uses when WithRetry is not given:
// three attempts, 500ms base, capped at 30s, with full jitter.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    30 * time.Second,
	}
}

// delay computes how long to wait before attempt number n (1-based: the wait
// before the second attempt is delay(1)). retryAfter, when non-zero, is the
// server's request and wins over the computed backoff, still capped by
// MaxDelay.
func (p RetryPolicy) delay(n int, retryAfter time.Duration) time.Duration {
	maxDelay := p.MaxDelay
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}

	if retryAfter > 0 {
		return min(retryAfter, maxDelay)
	}

	d := p.BaseDelay
	if d <= 0 {
		return 0
	}
	for range n - 1 {
		d *= 2
		if d >= maxDelay {
			return p.jitter(maxDelay)
		}
	}
	return p.jitter(min(d, maxDelay))
}

func (p RetryPolicy) jitter(d time.Duration) time.Duration {
	if p.Jitter != nil {
		return p.Jitter(d)
	}
	if d <= 0 {
		return 0
	}
	// Full jitter: uniformly random in [0, d). Not a security decision --
	// this only spreads retries out to avoid a thundering herd.
	return time.Duration(rand.Int64N(int64(d))) // #nosec G404
}
