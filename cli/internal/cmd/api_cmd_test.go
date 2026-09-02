package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAPICmd(t *testing.T) {
	t.Run("[happy] GET with --query pairs merged onto the path", func(t *testing.T) {
		setupCredentials(t)
		var gotPath, gotQuery string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
			// The path is /accounting/..., so the transport expects the
			// accounting family's enveloped shape.
			_, _ = w.Write([]byte(`{"response": {"result": {"ok": true}}}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"api", "GET", "/accounting/account/ACM000TEST/systems/systems/1?existing=1",
			"--query", "extra=probe", "--base-url", srv.URL}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if gotPath != "/accounting/account/ACM000TEST/systems/systems/1" {
			t.Errorf("path = %q", gotPath)
		}
		q, _ := url.ParseQuery(gotQuery)
		if q.Get("existing") != "1" || q.Get("extra") != "probe" {
			t.Errorf("query = %q, want both existing and extra params", gotQuery)
		}
		if !strings.Contains(stdout.String(), "\"ok\"") {
			t.Errorf("stdout = %q", stdout.String())
		}
	})

	t.Run("[happy] a JSON body from -f is sent on POST", func(t *testing.T) {
		setupCredentials(t)
		var gotBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(r.Body)
			gotBody = buf.Bytes()
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"api", "POST", "/accounting/account/ACM000TEST/users/clients",
			"-f", "-", "--base-url", srv.URL}
		stdin := strings.NewReader(`{"client":{"fname":"Ada"}}`)
		code := Run(args, stdin, &stdout, &stderr, "test")
		if code != 0 {
			t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(string(gotBody), "Ada") {
			t.Errorf("body = %s, want it to carry the stdin payload", gotBody)
		}
	})

	t.Run("[sad] a malformed --query pair is a usage error", func(t *testing.T) {
		setupCredentials(t)
		var stdout, stderr bytes.Buffer
		args := []string{"api", "GET", "/foo", "--query", "no-equals-sign", "--base-url", "http://127.0.0.1:1"}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 2 {
			t.Fatalf("exit = %d, want 2; stderr = %s", code, stderr.String())
		}
	})

	t.Run("[sad] an API error is surfaced with the right exit code", func(t *testing.T) {
		setupCredentials(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}))
		defer srv.Close()

		var stdout, stderr bytes.Buffer
		args := []string{"api", "GET", "/projects/business/9000001/projects/1", "--base-url", srv.URL}
		code := Run(args, discardStdin, &stdout, &stderr, "test")
		if code != 4 {
			t.Fatalf("exit = %d, want 4; stderr = %s", code, stderr.String())
		}
	})
}
