package freshbooks

import (
	"context"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestAttachmentsUploadExpenseReceipt(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a content part and decodes the full field set", func(t *testing.T) {
		var gotPath, gotFormName string
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
			gotFormName = part.FormName()
			serveFixture(t, http.StatusOK, "attachments", "upload")(w, r)
		}))

		got, err := c.Attachments.UploadExpenseReceipt(ctx, "ACM123", "receipt.jpg", strings.NewReader("fake-bytes"))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/uploads/account/ACM123/attachments" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotFormName != "content" {
			t.Fatalf("form name = %q", gotFormName)
		}
		if got.JWT == "" || got.MediaType != "image/jpeg" || got.UUID == "" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] the upload exceeds the size bound", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		big := strings.NewReader(strings.Repeat("a", maxUploadBytes+1))
		_, err := c.Attachments.UploadExpenseReceipt(ctx, "ACM123", "big.jpg", big)
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("err = %v", err)
		}
	})
}
