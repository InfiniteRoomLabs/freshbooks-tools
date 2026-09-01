package freshbooks

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

func TestDoMultipart(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] sends the file part, decodes a flat response", func(t *testing.T) {
		var gotFilename, gotFileContent, gotContentType string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotContentType = r.Header.Get("Content-Type")
			_, params, err := mime.ParseMediaType(gotContentType)
			if err != nil {
				t.Fatal(err)
			}
			mr := multipart.NewReader(r.Body, params["boundary"])
			part, err := mr.NextPart()
			if err != nil {
				t.Fatal(err)
			}
			gotFilename = part.FileName()
			gotFileContent = string(mustReadAll(t, part))
			_, _ = io.WriteString(w, `{"image": {"jwt": "abc"}}`)
		})
		c, _ := newTestClient(t, h)

		var out struct {
			Image struct {
				JWT string `json:"jwt"`
			} `json:"image"`
		}
		err := c.doMultipart(ctx, http.MethodPost, "/uploads/account/ACM123/images", FamilyBusiness,
			"logo", "toronto-skyline.jpg", strings.NewReader("fake-image-bytes"), &out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(gotContentType, "multipart/form-data") {
			t.Fatalf("Content-Type = %q", gotContentType)
		}
		if gotFilename != "toronto-skyline.jpg" || gotFileContent != "fake-image-bytes" {
			t.Fatalf("filename = %q, content = %q", gotFilename, gotFileContent)
		}
		if out.Image.JWT != "abc" {
			t.Fatalf("out = %+v", out)
		}
	})

	t.Run("[happy] the filename is reduced to its base name", func(t *testing.T) {
		var gotFilename string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				t.Fatal(err)
			}
			mr := multipart.NewReader(r.Body, params["boundary"])
			part, err := mr.NextPart()
			if err != nil {
				t.Fatal(err)
			}
			gotFilename = part.FileName()
			_, _ = io.WriteString(w, `{}`)
		}))
		err := c.doMultipart(ctx, http.MethodPost, "/uploads/account/ACM123/images", FamilyBusiness,
			"logo", "../../etc/passwd", strings.NewReader("x"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if gotFilename != "passwd" {
			t.Fatalf("filename = %q, want the base name only", gotFilename)
		}
	})

	t.Run("[sad] a server error decodes as a flat business error", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error": "invalid_file", "error_description": "not an image"}`)
		}))
		err := c.doMultipart(ctx, http.MethodPost, "/uploads/account/ACM123/images", FamilyBusiness,
			"logo", "x.jpg", strings.NewReader("x"), nil)
		if err == nil || !strings.Contains(err.Error(), "not an image") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] the upload exceeds the size bound", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		big := strings.NewReader(strings.Repeat("a", maxUploadBytes+1))
		err := c.doMultipart(ctx, http.MethodPost, "/uploads/account/ACM123/images", FamilyBusiness,
			"logo", "x.jpg", big, nil)
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("err = %v, want an oversize error", err)
		}
	})

	t.Run("[happy] a transient failure is retried and the same bytes are replayed", func(t *testing.T) {
		var hits atomic.Int32
		var bodies []string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			bodies = append(bodies, string(b))
			if hits.Add(1) == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = io.WriteString(w, `{"image": {"jwt": "abc"}}`)
		})
		c, _ := newTestClient(t, h, WithRetry(testRetry(2)))
		err := c.doMultipart(ctx, http.MethodPost, "/uploads/account/ACM123/images", FamilyBusiness,
			"logo", "x.jpg", strings.NewReader("same-bytes"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(bodies) != 2 || bodies[0] != bodies[1] {
			t.Fatalf("bodies did not replay identically: %v", bodies)
		}
	})
}

func mustReadAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// recordingRoundTripper answers every request with a canned response and
// keeps the last request it saw, so tests can inspect exactly what
// doOnHost built (scheme, host, query) without a real listener.
type recordingRoundTripper struct {
	req    *http.Request
	resp   string
	status int
}

func (rt *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.req = req
	status := rt.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(rt.resp)),
		Header:     make(http.Header),
	}, nil
}

func newRecordingClient(t *testing.T, baseURL string, rt *recordingRoundTripper) *Client {
	t.Helper()
	c, err := NewClient(
		WithBaseURL(baseURL),
		WithHTTPClient(&http.Client{Transport: rt}),
		WithTokenSource(auth.StaticTokenSource("test-access-token")),
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestDoOnHost(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] forces https and the override host regardless of the base URL's http scheme", func(t *testing.T) {
		rt := &recordingRoundTripper{resp: `{"cc_token": "tok_123"}`}
		// The base URL is deliberately http:// -- doOnHost must not honor
		// it for the scheme, only borrow it as a starting point.
		c := newRecordingClient(t, "http://api.freshbooks.test", rt)

		var out struct {
			CCToken string `json:"cc_token"`
		}
		err := c.doOnHost(ctx, http.MethodPost, "paid.freshbooks.test", "/gateway/fbpay/tokenize", FamilyBusiness,
			map[string]string{"card_number": "4111111111111111"}, &out)
		if err != nil {
			t.Fatal(err)
		}
		if rt.req.URL.Scheme != "https" {
			t.Fatalf("scheme = %q, want https", rt.req.URL.Scheme)
		}
		if rt.req.URL.Host != "paid.freshbooks.test" {
			t.Fatalf("host = %q", rt.req.URL.Host)
		}
		if rt.req.URL.Path != "/gateway/fbpay/tokenize" {
			t.Fatalf("path = %q", rt.req.URL.Path)
		}
		if rt.req.Header.Get("Authorization") != "Bearer test-access-token" {
			t.Fatalf("Authorization = %q", rt.req.Header.Get("Authorization"))
		}
		if out.CCToken != "tok_123" {
			t.Fatalf("out = %+v", out)
		}
	})

	t.Run("[happy] the base URL's own query string is dropped, not appended", func(t *testing.T) {
		rt := &recordingRoundTripper{resp: `{}`}
		c := newRecordingClient(t, "http://api.freshbooks.test/v1?leftover=1", rt)
		if err := c.doOnHost(ctx, http.MethodPost, "paid.freshbooks.test", "/gateway/fbpay/tokenize", FamilyBusiness, nil, nil); err != nil {
			t.Fatal(err)
		}
		if rt.req.URL.RawQuery != "" {
			t.Fatalf("query = %q, want empty", rt.req.URL.RawQuery)
		}
	})

	t.Run("[sad] a directory traversal in path is rejected before any request is sent", func(t *testing.T) {
		rt := &recordingRoundTripper{resp: `{}`}
		c := newRecordingClient(t, "http://api.freshbooks.test", rt)
		if err := c.doOnHost(ctx, http.MethodPost, "paid.freshbooks.test", "/gateway/../secret", FamilyBusiness, nil, nil); err == nil {
			t.Fatal("want an error")
		}
		if rt.req != nil {
			t.Fatal("a request was made with a traversal path")
		}
	})

	t.Run("[sad] a server error surfaces normally", func(t *testing.T) {
		rt := &recordingRoundTripper{resp: `{"error": "invalid_card"}`, status: http.StatusUnprocessableEntity}
		c := newRecordingClient(t, "http://api.freshbooks.test", rt)
		err := c.doOnHost(ctx, http.MethodPost, "paid.freshbooks.test", "/gateway/fbpay/tokenize", FamilyBusiness, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid_card") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestFetchRaw(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] returns the body unparsed and honors a custom Accept header", func(t *testing.T) {
		var gotAccept string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAccept = r.Header.Get("Accept")
			w.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(w, "a,b\n1,2\n")
		}))
		got, err := c.fetchRaw(ctx, http.MethodGet, "/accounting/account/ACM123/links/reports/tok/invoice_details.csv", FamilyAccounting, "text/csv")
		if err != nil {
			t.Fatal(err)
		}
		if gotAccept != "text/csv" {
			t.Fatalf("Accept = %q, want text/csv", gotAccept)
		}
		if string(got) != "a,b\n1,2\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("[happy] an empty accept keeps the default application/json", func(t *testing.T) {
		var gotAccept string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAccept = r.Header.Get("Accept")
			_, _ = io.WriteString(w, `{}`)
		}))
		if _, err := c.fetchRaw(ctx, http.MethodGet, "/accounting/account/ACM123/x", FamilyAccounting, ""); err != nil {
			t.Fatal(err)
		}
		if gotAccept != "application/json" {
			t.Fatalf("Accept = %q", gotAccept)
		}
	})

	t.Run("[sad] a server error is still decoded as an API error", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		_, err := c.fetchRaw(ctx, http.MethodGet, "/accounting/account/ACM123/links/reports/tok/invoice_details.csv", FamilyAccounting, "text/csv")
		if err == nil {
			t.Fatal("want an error")
		}
	})
}
