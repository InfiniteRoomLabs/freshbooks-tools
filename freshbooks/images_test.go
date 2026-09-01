package freshbooks

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestImagesUpload(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a content part to the account-scoped path", func(t *testing.T) {
		var gotPath, gotFormName, gotFilename string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				t.Fatal(err)
			}
			mr := multipart.NewReader(r.Body, params["boundary"])
			part, err := mr.NextPart()
			if err != nil {
				t.Fatal(err)
			}
			gotFormName, gotFilename = part.FormName(), part.FileName()
			serveFixture(t, http.StatusOK, "images", "upload")(w, r)
		}))

		got, err := c.Images.Upload(ctx, "ACM123", "toronto-skyline.jpg", strings.NewReader("fake-bytes"))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/uploads/account/ACM123/images" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotFormName != "content" || gotFilename != "toronto-skyline.jpg" {
			t.Fatalf("form name = %q, filename = %q", gotFormName, gotFilename)
		}
		if got.JWT == "" || got.Filename == "" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] the server rejects the file", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error": "invalid_file"}`)
		}))
		_, err := c.Images.Upload(ctx, "ACM123", "x.jpg", strings.NewReader("x"))
		if err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestImagesUploadWithoutAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts to the account-less path", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "images", "upload")(w, r)
		}))
		got, err := c.Images.UploadWithoutAccount(ctx, "app-logo.png", strings.NewReader("fake-bytes"))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/uploads/images" {
			t.Fatalf("path = %q", gotPath)
		}
		if got.JWT == "" {
			t.Fatalf("got = %+v", got)
		}
	})
}
