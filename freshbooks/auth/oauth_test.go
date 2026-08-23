package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fixture returns the bytes of ../testdata/<area>/<name>.json.
func fixture(t *testing.T, area, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", area, name+".json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return b
}

// testConfig returns a Config pointing every endpoint at srv.
func testConfig(srv *httptest.Server) Config {
	return Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "https://localhost:8765/callback",
		Scopes:       []string{"user:profile:read", "user:clients:read"},
		Endpoints: Endpoints{
			AuthURL:   srv.URL + "/authorize",
			TokenURL:  srv.URL + "/token",
			RevokeURL: srv.URL + "/revoke",
		},
		HTTPClient: srv.Client(),
		Now:        func() time.Time { return time.Unix(1756000000, 0).UTC() },
	}
}

func TestEndpointDefaults(t *testing.T) {
	t.Run("[happy] the zero Endpoints value means the RFC 8414 set", func(t *testing.T) {
		if got := (Config{}).endpoints(); got != MetadataEndpoints {
			t.Fatalf("endpoints() = %+v, want %+v", got, MetadataEndpoints)
		}
	})
	t.Run("[happy] an explicit set is honoured", func(t *testing.T) {
		c := Config{Endpoints: DocumentedEndpoints}
		if got := c.endpoints(); got != DocumentedEndpoints {
			t.Fatalf("endpoints() = %+v", got)
		}
	})
	t.Run("[happy] defaults for client and clock", func(t *testing.T) {
		c := Config{}
		if c.httpClient() != http.DefaultClient {
			t.Fatal("httpClient() should default to http.DefaultClient")
		}
		if c.now().IsZero() {
			t.Fatal("now() should default to time.Now")
		}
	})
}

func TestNewVerifierAndChallenge(t *testing.T) {
	v1, err := NewVerifier()
	if err != nil {
		t.Fatal(err)
	}
	v2, err := NewVerifier()
	if err != nil {
		t.Fatal(err)
	}
	if v1 == v2 {
		t.Fatal("two verifiers must not be equal")
	}
	if len(v1) != 43 {
		t.Fatalf("verifier length = %d, want 43", len(v1))
	}
	if _, err := base64.RawURLEncoding.DecodeString(v1); err != nil {
		t.Fatalf("verifier is not base64url: %v", err)
	}

	sum := sha256.Sum256([]byte(v1))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); Challenge(v1) != want {
		t.Fatalf("Challenge = %q, want %q", Challenge(v1), want)
	}
}

func TestAuthCodeURL(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	cfg := testConfig(srv)

	t.Run("[happy] carries PKCE S256 and every required parameter", func(t *testing.T) {
		raw, verifier, err := cfg.AuthCodeURL("state-123")
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		want := map[string]string{
			"response_type":         "code",
			"client_id":             "test-client-id",
			"redirect_uri":          "https://localhost:8765/callback",
			"state":                 "state-123",
			"code_challenge_method": "S256",
			"scope":                 "user:profile:read user:clients:read",
			"code_challenge":        Challenge(verifier),
		}
		for k, v := range want {
			if got := q.Get(k); got != v {
				t.Errorf("%s = %q, want %q", k, got, v)
			}
		}
		if strings.Contains(raw, verifier) {
			t.Error("the authorization URL must never carry the code verifier")
		}
	})

	t.Run("[edge] no scopes means no scope parameter", func(t *testing.T) {
		c := cfg
		c.Scopes = nil
		raw, _, err := c.AuthCodeURL("s")
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(raw)
		if _, ok := u.Query()["scope"]; ok {
			t.Fatal("scope should be absent")
		}
	})

	t.Run("[edge] existing query parameters on the endpoint survive", func(t *testing.T) {
		c := cfg
		c.Endpoints.AuthURL = srv.URL + "/authorize?tenant=eu"
		raw, _, err := c.AuthCodeURL("s")
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(raw)
		if u.Query().Get("tenant") != "eu" {
			t.Fatalf("lost the endpoint's own query: %s", raw)
		}
	})

	t.Run("[sad] an unparseable endpoint is an error", func(t *testing.T) {
		c := cfg
		c.Endpoints.AuthURL = "://nope"
		if _, _, err := c.AuthCodeURL("s"); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestExchange(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture(t, "auth", "token"))
	}))
	defer srv.Close()

	tok, err := testConfig(srv).Exchange(context.Background(), "auth-code-1", "verifier-1")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("[happy] posts the authorization-code grant with the PKCE verifier", func(t *testing.T) {
		want := map[string]string{
			"grant_type":    "authorization_code",
			"client_id":     "test-client-id",
			"client_secret": "test-client-secret",
			"code":          "auth-code-1",
			"redirect_uri":  "https://localhost:8765/callback",
			"code_verifier": "verifier-1",
		}
		for k, v := range want {
			if got := gotForm.Get(k); got != v {
				t.Errorf("form[%s] = %q, want %q", k, got, v)
			}
		}
	})

	t.Run("[happy] expiry is created_at plus expires_in, not now plus expires_in", func(t *testing.T) {
		want := time.Unix(1756000000, 0).UTC().Add(43200 * time.Second)
		if !tok.Expiry.Equal(want) {
			t.Fatalf("Expiry = %v, want %v", tok.Expiry, want)
		}
		if tok.AccessToken == "" || tok.RefreshToken == "" {
			t.Fatal("token pair incomplete")
		}
		if len(tok.Scopes) != 2 || tok.Scopes[0] != "user:profile:read" {
			t.Fatalf("Scopes = %v", tok.Scopes)
		}
	})
}

func TestTokenEndpointErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       []byte
		wantCode   string
		wantIs     error
		wantErrStr string
	}{
		{
			name:     "[sad] invalid_grant on a reused refresh token",
			status:   http.StatusBadRequest,
			body:     fixture(t, "auth", "token_error"),
			wantCode: "invalid_grant",
			wantIs:   ErrInvalidGrant,
		},
		{
			name:       "[sad] an unparseable error body still reports the status",
			status:     http.StatusInternalServerError,
			body:       []byte("<html>oops</html>"),
			wantErrStr: "freshbooks/auth: 500 Internal Server Error",
		},
		{
			name:       "[edge] a code with no description",
			status:     http.StatusBadRequest,
			body:       []byte(`{"error": "invalid_request"}`),
			wantCode:   "invalid_request",
			wantErrStr: "freshbooks/auth: 400 invalid_request",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write(tc.body)
			}))
			defer srv.Close()

			_, err := testConfig(srv).Refresh(context.Background(), "refresh-1")
			var authErr *Error
			if !errors.As(err, &authErr) {
				t.Fatalf("err = %v, want an *auth.Error", err)
			}
			if authErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", authErr.StatusCode, tc.status)
			}
			if tc.wantCode != "" && authErr.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", authErr.Code, tc.wantCode)
			}
			if tc.wantIs != nil && !errors.Is(err, tc.wantIs) {
				t.Errorf("errors.Is(err, %v) = false", tc.wantIs)
			}
			if tc.wantErrStr != "" && authErr.Error() != tc.wantErrStr {
				t.Errorf("Error() = %q, want %q", authErr.Error(), tc.wantErrStr)
			}
			if strings.Contains(err.Error(), "test-client-secret") {
				t.Error("the client secret must never reach an error string")
			}
		})
	}

	t.Run("[sad] an unmapped code has no sentinel", func(t *testing.T) {
		e := &Error{StatusCode: 400, Code: "invalid_request"}
		if e.Unwrap() != nil {
			t.Fatal("want no sentinel")
		}
	})
}

func TestTokenResponseValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"[sad] malformed JSON", `{not json`},
		{"[sad] no access_token", `{"token_type": "Bearer"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			if _, err := testConfig(srv).Exchange(context.Background(), "c", "v"); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestPostFormTransportFailures(t *testing.T) {
	t.Run("[sad] a cancelled context aborts the request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(fixture(t, "auth", "token"))
		}))
		defer srv.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := testConfig(srv).Exchange(ctx, "c", "v")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})

	t.Run("[corner] an oversized response body is rejected", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token": "` + strings.Repeat("a", maxTokenResponseBytes) + `"}`))
		}))
		defer srv.Close()

		_, err := testConfig(srv).Exchange(context.Background(), "c", "v")
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("err = %v, want an oversize error", err)
		}
	})

	t.Run("[sad] an unroutable endpoint reports the endpoint without its query", func(t *testing.T) {
		cfg := Config{Endpoints: Endpoints{TokenURL: "http://127.0.0.1:1/token?code=secret-code"}}
		_, err := cfg.Exchange(context.Background(), "c", "v")
		if err == nil {
			t.Fatal("want an error")
		}
		if strings.Contains(err.Error(), "secret-code") {
			t.Fatalf("the query string leaked into %q", err)
		}
	})

	t.Run("[sad] an unbuildable request is an error", func(t *testing.T) {
		cfg := Config{Endpoints: Endpoints{TokenURL: "://nope"}}
		if _, err := cfg.Exchange(context.Background(), "c", "v"); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestEndpointName(t *testing.T) {
	tests := map[string]string{
		"https://example.test/token?code=abc": "https://example.test/token",
		"https://u:p@example.test/token":      "https://example.test/token",
		"://nope":                             "the OAuth endpoint",
	}
	for in, want := range tests {
		if got := endpointName(in); got != want {
			t.Errorf("endpointName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRevoke(t *testing.T) {
	var hits atomic.Int32
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(body))
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	t.Run("[happy] posts client credentials and the token", func(t *testing.T) {
		if err := testConfig(srv).Revoke(context.Background(), "refresh-token-1"); err != nil {
			t.Fatal(err)
		}
		if gotForm.Get("token") != "refresh-token-1" || gotForm.Get("client_id") != "test-client-id" {
			t.Fatalf("form = %v", gotForm)
		}
		if hits.Load() != 1 {
			t.Fatalf("hits = %d", hits.Load())
		}
	})

	t.Run("[sad] a failing revoke surfaces the status", func(t *testing.T) {
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error": "invalid_client"}`)
		}))
		defer bad.Close()
		err := testConfig(bad).Revoke(context.Background(), "t")
		var authErr *Error
		if !errors.As(err, &authErr) || authErr.StatusCode != http.StatusUnauthorized {
			t.Fatalf("err = %v", err)
		}
	})
}
