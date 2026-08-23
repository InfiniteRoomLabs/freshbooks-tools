package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

func TestRequestHeadersAndBody(t *testing.T) {
	var (
		gotAuth, gotUA, gotAccept, gotContentType, gotMethod, gotQuery string
		gotBody                                                        []byte
	)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotContentType = r.Header.Get("Content-Type")
		gotMethod = r.Method
		gotQuery = r.URL.RawQuery
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = io.WriteString(w, `{"response": {"result": {"ok": true}}}`)
	})
	c, _ := newTestClient(t, h, WithUserAgent("test-agent/9"))

	var out struct {
		OK bool `json:"ok"`
	}
	err := c.do(context.Background(), http.MethodPost, "/accounting/account/ACM123/users/clients",
		FamilyAccounting, map[string]string{"hello": "world"}, &out,
		Include("lines"), Search{"status": "paid"}, PageNumber(2), PerPage(5), Sort("created", SortDesc))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("[happy] headers", func(t *testing.T) {
		if gotAuth != "Bearer test-access-token" {
			t.Errorf("Authorization = %q", gotAuth)
		}
		if gotUA != "test-agent/9" {
			t.Errorf("User-Agent = %q", gotUA)
		}
		if gotAccept != "application/json" || gotContentType != "application/json" {
			t.Errorf("Accept = %q, Content-Type = %q", gotAccept, gotContentType)
		}
		if gotMethod != http.MethodPost {
			t.Errorf("method = %q", gotMethod)
		}
	})
	t.Run("[happy] JSON body", func(t *testing.T) {
		if strings.TrimSpace(string(gotBody)) != `{"hello":"world"}` {
			t.Errorf("body = %s", gotBody)
		}
	})
	t.Run("[happy] accounting query encoding", func(t *testing.T) {
		want := "include%5B%5D=lines&page=2&per_page=5&search%5Bstatus%5D=paid&sort=created_desc"
		if gotQuery != want {
			t.Errorf("query = %q, want %q", gotQuery, want)
		}
	})
	t.Run("[happy] envelope unwrapped", func(t *testing.T) {
		if !out.OK {
			t.Error("the result envelope was not unwrapped")
		}
	})
}

func TestNoTokenSourceSendsNoAuthorization(t *testing.T) {
	var seen bool
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, seen = r.Header["Authorization"]
		_, _ = io.WriteString(w, `{}`)
	})
	srv := httptest.NewServer(h)
	defer srv.Close()

	c, err := NewClient(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Do(context.Background(), http.MethodGet, "/projects/business/1/projects", nil, nil); err != nil {
		t.Fatal(err)
	}
	if seen {
		t.Fatal("an Authorization header was sent without a token source")
	}
}

func TestEnvelopeUnwrapping(t *testing.T) {
	tests := []struct {
		name   string
		family Family
		path   string
		body   string
		want   string
	}{
		{
			name:   "[happy] accounting peels response.result",
			family: FamilyAccounting,
			path:   "/accounting/account/ACM123/users/clients",
			body:   `{"response": {"result": {"total": 7}}}`,
			want:   `{"total": 7}`,
		},
		{
			name:   "[happy] auth peels response only",
			family: FamilyAuth,
			path:   "/auth/api/v1/users/me",
			body:   `{"response": {"id": 42}}`,
			want:   `{"id": 42}`,
		},
		{
			name:   "[happy] business-scoped bodies are flat",
			family: FamilyBusiness,
			path:   "/projects/business/1/projects",
			body:   `{"meta": {"total": 0}, "projects": []}`,
			want:   `{"meta": {"total": 0}, "projects": []}`,
		},
		{
			name:   "[edge] an accounting response with no result layer is passed through",
			family: FamilyAccounting,
			path:   "/accounting/account/ACM123/users/clients",
			body:   `{"response": {"id": 1}}`,
			want:   `{"id": 1}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := unwrap([]byte(tc.body), tc.family)
			if err != nil {
				t.Fatal(err)
			}
			var gotAny, wantAny any
			if err := json.Unmarshal(got, &gotAny); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &wantAny); err != nil {
				t.Fatal(err)
			}
			if !jsonEqual(gotAny, wantAny) {
				t.Fatalf("unwrapped %s, want %s", got, tc.want)
			}
		})
	}

	t.Run("[edge] an empty envelope yields no payload", func(t *testing.T) {
		got, err := unwrap([]byte(`{}`), FamilyAccounting)
		if err != nil || got != nil {
			t.Fatalf("got %s, %v", got, err)
		}
	})

	t.Run("[sad] malformed JSON", func(t *testing.T) {
		if _, err := unwrap([]byte(`{nope`), FamilyAccounting); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a non-object response member", func(t *testing.T) {
		if _, err := unwrap([]byte(`{"response": 3}`), FamilyAccounting); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[edge] decodeBody with a nil out discards the payload", func(t *testing.T) {
		if err := decodeBody([]byte(`{"response": {"result": {}}}`), FamilyAccounting, nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("[edge] a null payload leaves out untouched", func(t *testing.T) {
		out := map[string]any{"kept": true}
		if err := decodeBody([]byte(`{"response": {"result": null}}`), FamilyAccounting, &out); err != nil {
			t.Fatal(err)
		}
		if !out["kept"].(bool) {
			t.Fatal("out was overwritten")
		}
	})

	t.Run("[sad] a payload that does not fit out", func(t *testing.T) {
		var out struct {
			Total int `json:"total"`
		}
		err := decodeBody([]byte(`{"response": {"result": {"total": "seven"}}}`), FamilyAccounting, &out)
		if err == nil || !strings.Contains(err.Error(), "decoding the response") {
			t.Fatalf("err = %v", err)
		}
	})
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		area   string
		file   string
		path   string
		wantIs error
		wantMs string
	}{
		{"[sad] 401", 401, "auth", "error_401", "/auth/api/v1/users/me", ErrUnauthorized, "unauthenticated"},
		{"[sad] 404 accounting", 404, "accounting", "error_404", "/accounting/account/ACM123/users/clients/1", ErrNotFound, "Client not found."},
		{"[sad] 404 business-scoped", 404, "projects", "error_404", "/projects/business/1/projects/1", ErrNotFound, "Requested resource could not be found."},
		{"[sad] 422", 422, "accounting", "error_422", "/accounting/account/ACM123/users/clients", ErrValidation, "Validation failed."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newTestClient(t, serveFixture(t, tc.status, tc.area, tc.file))
			err := c.Do(context.Background(), http.MethodGet, tc.path, nil, &struct{}{})
			if !errors.Is(err, tc.wantIs) {
				t.Fatalf("err = %v, want %v", err, tc.wantIs)
			}
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v, want an *Error", err)
			}
			if !strings.Contains(apiErr.Message, tc.wantMs) {
				t.Fatalf("Message = %q, want it to contain %q", apiErr.Message, tc.wantMs)
			}
			if strings.Contains(err.Error(), "test-access-token") {
				t.Fatal("the access token reached an error string")
			}
		})
	}

	t.Run("[sad] a 429 carries Retry-After", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"response": {"errors": [{"errno": 429, "message": "slow down"}]}}`)
		})
		c, _ := newTestClient(t, h)
		err := c.Do(context.Background(), http.MethodGet, "/accounting/account/ACM123/users/clients", nil, nil)
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			t.Fatalf("err = %v", err)
		}
		if !errors.Is(err, ErrRateLimited) {
			t.Fatal("want ErrRateLimited")
		}
		if apiErr.RetryAfter() != 7*time.Second {
			t.Fatalf("RetryAfter = %v", apiErr.RetryAfter())
		}
	})

	t.Run("[edge] the error's family survives a base URL path prefix", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"response": {"errors": [{"errno": 1012, "message": "gone"}]}}`)
		}))
		defer srv.Close()

		// With a prefix, re-deriving the family from the request path would
		// see "/v1/accounting/..." and mislabel every error as business.
		c, err := NewClient(WithBaseURL(srv.URL+"/v1"), WithHTTPClient(srv.Client()))
		if err != nil {
			t.Fatal(err)
		}
		err = c.Do(context.Background(), http.MethodGet, "/accounting/account/ACM123/users/clients/1", nil, nil)
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			t.Fatalf("err = %v", err)
		}
		if apiErr.Family != FamilyAccounting {
			t.Fatalf("Family = %q, want %q", apiErr.Family, FamilyAccounting)
		}
	})

	t.Run("[sad] malformed success JSON", func(t *testing.T) {
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"response": {"result": `)
		})
		c, _ := newTestClient(t, h)
		err := c.Do(context.Background(), http.MethodGet, "/accounting/account/ACM123/users/clients", nil, &struct{}{})
		if err == nil || !strings.Contains(err.Error(), "envelope") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRetry(t *testing.T) {
	ctx := context.Background()

	for _, status := range []int{429, 502, 503, 504} {
		t.Run("[happy] retries "+http.StatusText(status)+" and then succeeds", func(t *testing.T) {
			var hits atomic.Int32
			h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if hits.Add(1) == 1 {
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"error": "transient"}`)
					return
				}
				_, _ = io.WriteString(w, `{"response": {"result": {"ok": true}}}`)
			})
			c, _ := newTestClient(t, h, WithRetry(testRetry(3)))

			var out struct {
				OK bool `json:"ok"`
			}
			if err := c.Do(ctx, http.MethodGet, "/accounting/account/ACM123/users/clients", nil, &out); err != nil {
				t.Fatal(err)
			}
			if !out.OK || hits.Load() != 2 {
				t.Fatalf("ok = %v after %d attempts", out.OK, hits.Load())
			}
		})
	}

	t.Run("[sad] retry exhaustion returns the last error", func(t *testing.T) {
		var hits atomic.Int32
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{"error": "still down"}`)
		})
		c, _ := newTestClient(t, h, WithRetry(testRetry(3)))

		err := c.Do(ctx, http.MethodGet, "/accounting/account/ACM123/users/clients", nil, nil)
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("err = %v", err)
		}
		if hits.Load() != 3 {
			t.Fatalf("attempted %d times, want 3", hits.Load())
		}
	})

	t.Run("[happy] a non-retryable status is attempted once", func(t *testing.T) {
		var hits atomic.Int32
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error": "nope"}`)
		})
		c, _ := newTestClient(t, h, WithRetry(testRetry(3)))
		if err := c.Do(ctx, http.MethodGet, "/accounting/account/ACM123/users/clients", nil, nil); err == nil {
			t.Fatal("want an error")
		}
		if hits.Load() != 1 {
			t.Fatalf("attempted %d times, want 1", hits.Load())
		}
	})

	t.Run("[happy] the request body is replayed on a retry", func(t *testing.T) {
		var bodies []string
		var mu atomic.Int32
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			bodies = append(bodies, string(b))
			if mu.Add(1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = io.WriteString(w, `{"response": {"result": {}}}`)
		})
		c, _ := newTestClient(t, h, WithRetry(testRetry(2)))
		if err := c.Do(ctx, http.MethodPost, "/accounting/account/ACM123/users/clients", map[string]int{"n": 1}, nil); err != nil {
			t.Fatal(err)
		}
		if len(bodies) != 2 || bodies[0] != bodies[1] || bodies[0] != `{"n":1}` {
			t.Fatalf("bodies = %q", bodies)
		}
	})

	t.Run("[sad] a transport failure is retried and then surfaced", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL
		srv.Close() // nothing is listening now

		c, err := NewClient(WithBaseURL(url), WithRetry(testRetry(2)), WithClock(fixedClock))
		if err != nil {
			t.Fatal(err)
		}
		err = c.Do(ctx, http.MethodGet, "/projects/business/1/projects", nil, nil)
		if err == nil {
			t.Fatal("want an error")
		}
		if strings.Contains(err.Error(), "?") {
			t.Fatalf("the query string leaked into %q", err)
		}
	})

	t.Run("[sad] a cancelled context is not retried", func(t *testing.T) {
		var hits atomic.Int32
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			_, _ = io.WriteString(w, `{}`)
		})
		c, _ := newTestClient(t, h, WithRetry(testRetry(3)))

		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		err := c.Do(cancelled, http.MethodGet, "/projects/business/1/projects", nil, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if hits.Load() != 0 {
			t.Fatalf("the server was reached %d times", hits.Load())
		}
	})

	t.Run("[sad] a context cancelled during backoff stops the loop", func(t *testing.T) {
		cancellable, cancel := context.WithCancel(ctx)
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			cancel()
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, `{}`)
		})
		c, _ := newTestClient(t, h, WithRetry(RetryPolicy{MaxAttempts: 3, BaseDelay: time.Minute, MaxDelay: time.Minute}))

		start := time.Now()
		err := c.Do(cancellable, http.MethodGet, "/projects/business/1/projects", nil, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if time.Since(start) > 10*time.Second {
			t.Fatal("the backoff ignored the cancelled context")
		}
	})
}

func TestBoundedResponseBody(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"pad": "`)
		chunk := strings.Repeat("a", 1<<16)
		for range (maxResponseBytes / len(chunk)) + 2 {
			_, _ = io.WriteString(w, chunk)
		}
		_, _ = io.WriteString(w, `"}`)
	})
	c, _ := newTestClient(t, h)

	err := c.Do(context.Background(), http.MethodGet, "/projects/business/1/projects", nil, &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err = %v, want an oversize error", err)
	}
}

func TestRequestConstruction(t *testing.T) {
	c, _ := newTestClient(t, http.NotFoundHandler())
	ctx := context.Background()

	tests := []struct {
		name   string
		method string
		path   string
		body   any
		want   string
	}{
		{"[sad] an absolute path is refused", http.MethodGet, "https://evil.example.test/x", nil, "must be relative"},
		{"[sad] an unparseable path", http.MethodGet, "/x\x7f\x00", nil, "bad request path"},
		{"[sad] a body that cannot be marshalled", http.MethodPost, "/projects/business/1/projects", make(chan int), "encoding the request body"},
		{"[sad] an invalid method", "BAD METHOD", "/projects/business/1/projects", nil, "building the request"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := c.Do(ctx, tc.method, tc.path, tc.body, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}

	t.Run("[sad] a token source failure aborts before the request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler(), WithTokenSource(auth.NewTokenSource(auth.Config{}, auth.NewMemoryStore())))
		err := c.Do(ctx, http.MethodGet, "/projects/business/1/projects", nil, nil)
		if !errors.Is(err, auth.ErrNoToken) {
			t.Fatalf("err = %v, want auth.ErrNoToken", err)
		}
	})

	t.Run("[happy] a base URL with a path prefix is preserved", func(t *testing.T) {
		var gotPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_, _ = io.WriteString(w, `{}`)
		}))
		defer srv.Close()

		c, err := NewClient(WithBaseURL(srv.URL+"/v1"), WithHTTPClient(srv.Client()))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Do(ctx, http.MethodGet, "/projects/business/1/projects", nil, nil); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/v1/projects/business/1/projects" {
			t.Fatalf("path = %q", gotPath)
		}
	})

	t.Run("[happy] a path may carry its own query", func(t *testing.T) {
		var gotQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			_, _ = io.WriteString(w, `{}`)
		}))
		defer srv.Close()

		c, err := NewClient(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Do(ctx, http.MethodGet, "/projects/business/1/projects?state=active", nil, nil); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "state=active" {
			t.Fatalf("query = %q", gotQuery)
		}
	})
}

func TestRedactPath(t *testing.T) {
	tests := map[string]string{
		"https://api.example.test/x?token=abc": "https://api.example.test/x",
		"://nope":                              "the request URL",
	}
	for in, want := range tests {
		if got := redactPath(in); got != want {
			t.Errorf("redactPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWait(t *testing.T) {
	c, _ := newTestClient(t, http.NotFoundHandler())

	t.Run("[edge] a non-positive delay returns immediately", func(t *testing.T) {
		if err := c.wait(context.Background(), 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("[happy] a short delay elapses", func(t *testing.T) {
		if err := c.wait(context.Background(), time.Millisecond); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("[sad] a cancelled context short-circuits a zero delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := c.wait(ctx, 0); !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	})
}
