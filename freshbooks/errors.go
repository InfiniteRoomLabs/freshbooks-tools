package freshbooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Sentinel errors returned (via Unwrap) by *Error, for use with errors.Is.
var (
	// ErrUnauthorized is returned for HTTP 401: the access token is
	// missing, expired, or revoked.
	ErrUnauthorized = errors.New("freshbooks: unauthorized")
	// ErrForbidden is returned for HTTP 403: authenticated, but the token
	// lacks the scope or the account lacks the entitlement.
	ErrForbidden = errors.New("freshbooks: forbidden")
	// ErrNotFound is returned for HTTP 404.
	ErrNotFound = errors.New("freshbooks: not found")
	// ErrValidation is returned for HTTP 400 and 422.
	ErrValidation = errors.New("freshbooks: validation failed")
	// ErrRateLimited is returned for HTTP 429. Call Error.RetryAfter for
	// the server's requested delay.
	ErrRateLimited = errors.New("freshbooks: rate limited")
)

// Error is a decoded FreshBooks API error. The two API families report
// errors in different shapes; this type is what both normalize to.
//
// Error never renders the raw response body in its message, so an error from
// a token endpoint cannot leak a credential into a log line. Raw is kept for
// callers that explicitly want the original payload.
type Error struct {
	// StatusCode is the HTTP status of the failing response.
	StatusCode int
	// Code is the accounting family's "errno", or 0 when the family does
	// not supply one.
	Code int
	// Message is the human-readable error text.
	Message string
	// Field names the offending request field, when the API says.
	Field string
	// Family is the API family the request was addressed to.
	Family Family
	// Raw is the undecoded response body.
	Raw json.RawMessage

	retryAfter time.Duration
}

// Error implements the error interface.
func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("freshbooks: ")
	b.WriteString(strconv.Itoa(e.StatusCode))
	if e.Family != "" {
		b.WriteString(" ")
		b.WriteString(string(e.Family))
	}
	b.WriteString(": ")
	if e.Message != "" {
		b.WriteString(e.Message)
	} else {
		b.WriteString(http.StatusText(e.StatusCode))
	}
	var detail []string
	if e.Code != 0 {
		detail = append(detail, "errno "+strconv.Itoa(e.Code))
	}
	if e.Field != "" {
		detail = append(detail, "field "+e.Field)
	}
	if len(detail) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(detail, ", "))
		b.WriteString(")")
	}
	return b.String()
}

// Unwrap maps the HTTP status onto a sentinel so callers can use errors.Is.
func (e *Error) Unwrap() error {
	switch e.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ErrValidation
	case http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return nil
	}
}

// RetryAfter reports how long the server asked the caller to wait before
// retrying, or zero when it did not say.
func (e *Error) RetryAfter() time.Duration { return e.retryAfter }

// accountingErrorEnvelope is {"response": {"errors": [...]}}.
type accountingErrorEnvelope struct {
	Response struct {
		Errors []struct {
			Errno   int             `json:"errno"`
			Field   string          `json:"field"`
			Message string          `json:"message"`
			Object  string          `json:"object"`
			Value   json.RawMessage `json:"value"`
		} `json:"errors"`
	} `json:"response"`
}

// flatError covers every non-accounting error shape observed: the auth
// family's {"error", "error_description"}, the business family's bare
// {"error": "..."} string, and the {"message": ...} variant.
type flatError struct {
	Error       json.RawMessage `json:"error"`
	Description string          `json:"error_description"`
	Errno       int             `json:"errno"`
	Message     string          `json:"message"`
}

// decodeError turns a failing response body into an *Error. It never fails:
// an undecodable body still yields a usable error carrying the status.
func decodeError(status int, fam Family, body []byte, retryAfter time.Duration) *Error {
	e := &Error{
		StatusCode: status,
		Family:     fam,
		Raw:        append(json.RawMessage(nil), body...),
		retryAfter: retryAfter,
	}

	var env accountingErrorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && len(env.Response.Errors) > 0 {
		first := env.Response.Errors[0]
		e.Code, e.Field, e.Message = first.Errno, first.Field, first.Message
		return e
	}

	var flat flatError
	if err := json.Unmarshal(body, &flat); err == nil {
		e.Code = flat.Errno
		e.Message = flatMessage(flat)
		if e.Message != "" {
			return e
		}
	}

	e.Message = http.StatusText(status)
	return e
}

// flatMessage picks the most informative text out of a flat error body.
func flatMessage(f flatError) string {
	var code string
	if len(f.Error) > 0 {
		if err := json.Unmarshal(f.Error, &code); err != nil {
			code = strings.TrimSpace(string(f.Error))
		}
	}
	switch {
	case code != "" && f.Description != "":
		return fmt.Sprintf("%s: %s", code, f.Description)
	case code != "":
		return code
	case f.Description != "":
		return f.Description
	default:
		return f.Message
	}
}

// parseRetryAfter reads a Retry-After header in either of its RFC 9110
// spellings: delta-seconds, or an HTTP-date relative to now.
func parseRetryAfter(header string, now time.Time) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	if secs, err := strconv.Atoi(header); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(header); err == nil {
		if d := when.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
