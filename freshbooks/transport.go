package freshbooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxResponseBytes bounds what the transport will read from one response.
// FreshBooks list responses are measured in kilobytes; anything past this is
// a runaway server, and reading it would be an easy memory-exhaustion vector.
const maxResponseBytes = 10 << 20

// retryableStatuses are the HTTP statuses worth trying again: the documented
// rate limit plus the transient gateway failures.
var retryableStatuses = map[int]bool{
	http.StatusTooManyRequests:    true,
	http.StatusBadGateway:         true,
	http.StatusServiceUnavailable: true,
	http.StatusGatewayTimeout:     true,
}

// Do issues a request against an arbitrary API path using the same transport
// as every generated method: authentication, retry, envelope unwrapping, and
// family-specific error decoding. It is the escape hatch for endpoints this
// library does not model yet.
//
// path is rooted at the API base, e.g.
// "/accounting/account/ACM123/systems/systems/1". body is marshalled as JSON
// when non-nil; out receives the decoded payload when non-nil.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	return c.do(ctx, method, path, familyForPath(path), body, out)
}

// do is the single request path. Every service method funnels through it.
func (c *Client) do(ctx context.Context, method, path string, fam Family, body, out any, opts ...RequestOption) error {
	endpoint, err := c.resolve(path, fam, opts)
	if err != nil {
		return err
	}

	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("freshbooks: encoding the request body: %w", err)
		}
	}

	// The access token is resolved once for the whole request, not per
	// attempt: retries happen seconds apart, and a 401 is never retried, so
	// re-resolving would only risk spending a rotating refresh token again.
	authorization, err := c.authorization(ctx)
	if err != nil {
		return err
	}

	var lastErr error
	attempts := max(c.retry.MaxAttempts, 1)
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := c.newRequest(ctx, method, endpoint, payload, authorization)
		if err != nil {
			// A malformed method or URL fails identically every time.
			return err
		}
		raw, apiErr, err := c.roundTrip(ctx, req, fam, attempt)
		switch {
		case err != nil:
			lastErr = err
			if !isRetryableTransportError(err) {
				return err
			}
		case apiErr != nil:
			lastErr = apiErr
			if !retryableStatuses[apiErr.StatusCode] {
				return apiErr
			}
		default:
			return decodeBody(raw, fam, out)
		}

		if attempt == attempts {
			break
		}
		var retryAfter time.Duration
		var e *Error
		if errors.As(lastErr, &e) {
			retryAfter = e.RetryAfter()
		}
		if err := c.wait(ctx, c.retry.delay(attempt, retryAfter)); err != nil {
			return err
		}
	}
	return lastErr
}

// resolve builds the absolute request URL, applying the family's query
// encoding to opts.
func (c *Client) resolve(path string, fam Family, opts []RequestOption) (string, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("freshbooks: bad request path %q: %w", path, err)
	}
	if ref.IsAbs() {
		return "", fmt.Errorf("freshbooks: request path %q must be relative to the API base", path)
	}

	u := *c.baseURL
	u.Path = c.baseURL.Path + "/" + strings.TrimPrefix(ref.Path, "/")

	q := ref.Query()
	for key, vals := range newRequestOptions(opts).values(fam) {
		q[key] = append(q[key], vals...)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// roundTrip performs one HTTP round trip. Exactly one of the three results
// is meaningful: raw on success, apiErr for a decoded non-2xx response, err
// for anything that stopped the round trip.
func (c *Client) roundTrip(ctx context.Context, req *http.Request, fam Family, attempt int) (raw []byte, apiErr *Error, err error) {
	method, endpoint := req.Method, req.URL.String()

	c.logger.DebugContext(ctx, "freshbooks request", "method", method, "url", redactPath(endpoint), "attempt", attempt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// The *url.Error wrapper repeats the request URL; the cause alone
		// is enough and cannot carry a query string into a log.
		var uerr *url.Error
		if errors.As(err, &uerr) && uerr.Err != nil {
			err = uerr.Err
		}
		return nil, nil, fmt.Errorf("freshbooks: %s %s: %w", method, redactPath(endpoint), err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body, close error is not actionable

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("freshbooks: reading the response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, nil, fmt.Errorf("freshbooks: response exceeds the %d byte limit", maxResponseBytes)
	}

	c.logger.DebugContext(ctx, "freshbooks response", "method", method, "url", redactPath(endpoint), "status", resp.StatusCode, "attempt", attempt)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// fam comes from do(), not from req.URL.Path: a WithBaseURL path
		// prefix would make re-deriving it here disagree with the family
		// the request was actually built for.
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), c.now())
		return nil, decodeError(resp.StatusCode, fam, body, retryAfter), nil
	}
	return body, nil, nil
}

// authorization resolves the bearer header value, or "" when the client has
// no token source (fixture servers).
func (c *Client) authorization(ctx context.Context) (string, error) {
	if c.tokenSource == nil {
		return "", nil
	}
	tok, err := c.tokenSource.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("freshbooks: obtaining an access token: %w", err)
	}
	return "Bearer " + tok.AccessToken, nil
}

// newRequest builds one attempt's request, including a fresh body reader so
// a retry can replay it.
func (c *Client) newRequest(ctx context.Context, method, endpoint string, payload []byte, authorization string) (*http.Request, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("freshbooks: building the request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = int64(len(payload))
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	return req, nil
}

// wait sleeps for d, or returns early if ctx is done.
func (c *Client) wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
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

// isRetryableTransportError reports whether a failed round trip is worth
// repeating. Everything reaching it is network-level, so only a cancelled or
// expired context stops the loop.
//
// A network failure can happen after the server processed the request, so
// retrying one gives every method at-least-once semantics. See RetryPolicy
// for what that means for non-idempotent calls.
func isRetryableTransportError(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// accountingEnvelope is {"response": {"result": {...}}} and its auth-family
// cousin {"response": {...}}.
type accountingEnvelope struct {
	Response json.RawMessage `json:"response"`
}

// decodeBody unwraps the family's envelope and decodes into out. A nil out
// discards the payload.
func decodeBody(raw []byte, fam Family, out any) error {
	if out == nil {
		return nil
	}
	payload, err := unwrap(raw, fam)
	if err != nil {
		return err
	}
	if len(payload) == 0 || string(payload) == "null" {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("freshbooks: decoding the response: %w", err)
	}
	return nil
}

// unwrap peels the family's envelope off a successful response body.
//
// Accounting responses nest twice ({"response": {"result": {...}}}); the auth
// family nests once ({"response": {...}}) with no "result" layer; the
// business-scoped family is flat.
func unwrap(raw []byte, fam Family) (json.RawMessage, error) {
	if fam == FamilyBusiness {
		return raw, nil
	}

	var env accountingEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("freshbooks: decoding the response envelope: %w", err)
	}
	if len(env.Response) == 0 {
		// Some accounting endpoints answer a successful write with an
		// empty body or a bare object; treat that as no payload rather
		// than an error.
		return nil, nil
	}
	if fam == FamilyAuth {
		return env.Response, nil
	}

	var inner struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(env.Response, &inner); err != nil {
		return nil, fmt.Errorf("freshbooks: decoding the response envelope: %w", err)
	}
	if len(inner.Result) == 0 {
		return env.Response, nil
	}
	return inner.Result, nil
}

// redactPath renders a URL without its query string for error messages.
func redactPath(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "the request URL"
	}
	u.RawQuery, u.Fragment, u.User = "", "", nil
	return u.String()
}
