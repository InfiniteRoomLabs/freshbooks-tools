package freshbooks

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// readFixture returns the bytes of testdata/<area>/<name>.json.
func readFixture(t *testing.T, area, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", area, name+".json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return b
}

func TestDecodeError(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		family      Family
		body        []byte
		wantCode    int
		wantField   string
		wantMessage string
		wantIs      error
	}{
		{
			name:        "[happy] accounting 404 envelope with errno and field",
			status:      404,
			family:      FamilyAccounting,
			body:        readFixture(t, "accounting", "error_404"),
			wantCode:    1012,
			wantField:   "userid",
			wantMessage: "Client not found.",
			wantIs:      ErrNotFound,
		},
		{
			name:        "[happy] accounting 422 validation",
			status:      422,
			family:      FamilyAccounting,
			body:        readFixture(t, "accounting", "error_422"),
			wantCode:    2001,
			wantField:   "email",
			wantMessage: "Validation failed.",
			wantIs:      ErrValidation,
		},
		{
			name:        "[happy] business-scoped 404 is a bare error string",
			status:      404,
			family:      FamilyBusiness,
			body:        readFixture(t, "projects", "error_404"),
			wantMessage: "Requested resource could not be found.",
			wantIs:      ErrNotFound,
		},
		{
			name:        "[happy] 401 carries error and error_description",
			status:      401,
			family:      FamilyAuth,
			body:        readFixture(t, "auth", "error_401"),
			wantMessage: "unauthenticated: This action requires authentication to continue.",
			wantIs:      ErrUnauthorized,
		},
		{
			name:        "[edge] message-only body",
			status:      400,
			family:      FamilyBusiness,
			body:        []byte(`{"message": "bad request"}`),
			wantMessage: "bad request",
			wantIs:      ErrValidation,
		},
		{
			name:        "[edge] errno alongside a flat error string",
			status:      403,
			family:      FamilyBusiness,
			body:        []byte(`{"error": "forbidden", "errno": 4001}`),
			wantCode:    4001,
			wantMessage: "forbidden",
			wantIs:      ErrForbidden,
		},
		{
			name:        "[edge] non-string error member",
			status:      400,
			family:      FamilyBusiness,
			body:        []byte(`{"error": {"reason": "nope"}}`),
			wantMessage: `{"reason": "nope"}`,
			wantIs:      ErrValidation,
		},
		{
			name:        "[edge] description without a code",
			status:      400,
			family:      FamilyAuth,
			body:        []byte(`{"error_description": "just a description"}`),
			wantMessage: "just a description",
			wantIs:      ErrValidation,
		},
		{
			name:        "[sad] malformed JSON still yields a usable error",
			status:      500,
			family:      FamilyAccounting,
			body:        []byte(`{not json`),
			wantMessage: "Internal Server Error",
		},
		{
			name:        "[corner] empty accounting errors array falls through",
			status:      500,
			family:      FamilyAccounting,
			body:        []byte(`{"response": {"errors": []}}`),
			wantMessage: "Internal Server Error",
		},
		{
			name:        "[corner] 429 maps to the rate-limit sentinel",
			status:      429,
			family:      FamilyAccounting,
			body:        readFixture(t, "accounting", "error_429"),
			wantCode:    429,
			wantMessage: "Rate limit exceeded.",
			wantIs:      ErrRateLimited,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := decodeError(tc.status, tc.family, tc.body, 0)
			if e.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", e.StatusCode, tc.status)
			}
			if e.Family != tc.family {
				t.Errorf("Family = %q, want %q", e.Family, tc.family)
			}
			if e.Code != tc.wantCode {
				t.Errorf("Code = %d, want %d", e.Code, tc.wantCode)
			}
			if e.Field != tc.wantField {
				t.Errorf("Field = %q, want %q", e.Field, tc.wantField)
			}
			if e.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", e.Message, tc.wantMessage)
			}
			if string(e.Raw) != string(tc.body) {
				t.Errorf("Raw was not preserved")
			}
			if tc.wantIs != nil && !errors.Is(e, tc.wantIs) {
				t.Errorf("errors.Is(%v, %v) = false", e, tc.wantIs)
			}
		})
	}
}

func TestErrorString(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "[happy] full detail",
			err:  &Error{StatusCode: 404, Family: FamilyAccounting, Code: 1012, Field: "userid", Message: "Client not found."},
			want: "freshbooks: 404 accounting: Client not found. (errno 1012, field userid)",
		},
		{
			name: "[edge] no family, no detail",
			err:  &Error{StatusCode: 500, Message: "boom"},
			want: "freshbooks: 500: boom",
		},
		{
			name: "[edge] empty message falls back to the status text",
			err:  &Error{StatusCode: 503, Family: FamilyBusiness},
			want: "freshbooks: 503 business: Service Unavailable",
		},
		{
			name: "[edge] errno only",
			err:  &Error{StatusCode: 400, Code: 7, Message: "nope"},
			want: "freshbooks: 400: nope (errno 7)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestErrorUnwrapUnmapped(t *testing.T) {
	e := &Error{StatusCode: 418}
	if err := e.Unwrap(); err != nil {
		t.Fatalf("Unwrap() = %v, want nil", err)
	}
}

func TestErrorRetryAfter(t *testing.T) {
	e := decodeError(429, FamilyAccounting, []byte(`{}`), 3*time.Second)
	if got := e.RetryAfter(); got != 3*time.Second {
		t.Fatalf("RetryAfter() = %v", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"[happy] delta seconds", "30", 30 * time.Second},
		{"[happy] HTTP-date", now.Add(90 * time.Second).UTC().Format(http.TimeFormat), 90 * time.Second},
		{"[edge] empty", "", 0},
		{"[edge] zero seconds", "0", 0},
		{"[edge] negative seconds", "-5", 0},
		{"[edge] past HTTP-date", now.Add(-time.Hour).UTC().Format(http.TimeFormat), 0},
		{"[sad] garbage", "soon", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRetryAfter(tc.header, now); got != tc.want {
				t.Fatalf("parseRetryAfter(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}
