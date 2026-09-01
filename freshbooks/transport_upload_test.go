package freshbooks

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDoMultipart(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] sends the file part and fields, decodes a flat response", func(t *testing.T) {
		var gotFilename, gotFileContent, gotField, gotContentType string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotContentType = r.Header.Get("Content-Type")
			_, params, err := mime.ParseMediaType(gotContentType)
			if err != nil {
				t.Fatal(err)
			}
			mr := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				b, _ := io.ReadAll(part)
				switch part.FormName() {
				case "logo":
					gotFilename = part.FileName()
					gotFileContent = string(b)
				case "note":
					gotField = string(b)
				}
			}
			_, _ = io.WriteString(w, `{"image": {"jwt": "abc"}}`)
		})
		c, _ := newTestClient(t, h)

		var out struct {
			Image struct {
				JWT string `json:"jwt"`
			} `json:"image"`
		}
		err := c.doMultipart(ctx, http.MethodPost, "/uploads/account/ACM123/images", FamilyBusiness,
			"logo", "toronto-skyline.jpg", strings.NewReader("fake-image-bytes"), map[string]string{"note": "hi"}, &out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(gotContentType, "multipart/form-data") {
			t.Fatalf("Content-Type = %q", gotContentType)
		}
		if gotFilename != "toronto-skyline.jpg" || gotFileContent != "fake-image-bytes" {
			t.Fatalf("filename = %q, content = %q", gotFilename, gotFileContent)
		}
		if gotField != "hi" {
			t.Fatalf("note field = %q", gotField)
		}
		if out.Image.JWT != "abc" {
			t.Fatalf("out = %+v", out)
		}
	})

	t.Run("[sad] a server error decodes as a flat business error", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error": "invalid_file", "error_description": "not an image"}`)
		}))
		err := c.doMultipart(ctx, http.MethodPost, "/uploads/account/ACM123/images", FamilyBusiness,
			"logo", "x.jpg", strings.NewReader("x"), nil, nil)
		if err == nil || !strings.Contains(err.Error(), "not an image") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] the upload exceeds the size bound", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		big := strings.NewReader(strings.Repeat("a", maxUploadBytes+1))
		err := c.doMultipart(ctx, http.MethodPost, "/uploads/account/ACM123/images", FamilyBusiness,
			"logo", "x.jpg", big, nil, nil)
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
			"logo", "x.jpg", strings.NewReader("same-bytes"), nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(bodies) != 2 || bodies[0] != bodies[1] {
			t.Fatalf("bodies did not replay identically: %v", bodies)
		}
	})
}

func TestDoOnHost(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] the request is sent to the override host, not the base URL's", func(t *testing.T) {
		var gotHost, gotPath, gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotHost, gotPath, gotAuth = r.Host, r.URL.Path, r.Header.Get("Authorization")
			_, _ = io.WriteString(w, `{"cc_token": "tok_123"}`)
		}))
		defer srv.Close()

		// The base URL is srv.URL with a different, fake host in its
		// Host field; doOnHost must still land on the *actual* listener
		// because WithBaseURL's scheme+client point at srv, but the request
		// URL's host is swapped to the httptest server's real host.
		c, _ := newTestClient(t, http.NotFoundHandler())
		host := strings.TrimPrefix(srv.URL, "http://")

		var out struct {
			CCToken string `json:"cc_token"`
		}
		err := c.doOnHost(ctx, http.MethodPost, host, "/gateway/fbpay/tokenize", FamilyBusiness,
			map[string]string{"card_number": "4500123456789012"}, &out)
		if err != nil {
			t.Fatal(err)
		}
		if gotHost != host {
			t.Fatalf("Host = %q, want %q", gotHost, host)
		}
		if gotPath != "/gateway/fbpay/tokenize" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotAuth != "Bearer test-access-token" {
			t.Fatalf("Authorization = %q", gotAuth)
		}
		if out.CCToken != "tok_123" {
			t.Fatalf("out = %+v", out)
		}
	})

	t.Run("[sad] a server error surfaces normally", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"error": "invalid_card"}`)
		}))
		defer srv.Close()

		c, _ := newTestClient(t, http.NotFoundHandler())
		err := c.doOnHost(ctx, http.MethodPost, strings.TrimPrefix(srv.URL, "http://"), "/gateway/fbpay/tokenize", FamilyBusiness, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "invalid_card") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestDoRaw(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] returns the body unparsed", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(w, "a,b\n1,2\n")
		}))
		got, err := c.doRaw(ctx, http.MethodGet, "/accounting/account/ACM123/links/reports/tok/invoice_details.csv", FamilyAccounting)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "a,b\n1,2\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("[sad] a server error is still decoded as an API error", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		_, err := c.doRaw(ctx, http.MethodGet, "/accounting/account/ACM123/links/reports/tok/invoice_details.csv", FamilyAccounting)
		if err == nil {
			t.Fatal("want an error")
		}
	})
}
