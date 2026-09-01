package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestCallbacksRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts event/uri and decodes an unverified callback", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "callbacks", "register")(w, r)
		}))
		got, err := c.Callbacks.Register(ctx, "ACM123", &CallbackRegisterRequest{
			Event: "invoice.create", URI: "https://example.test/webhooks/ready",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/events/account/ACM123/events/callbacks" {
			t.Fatalf("path = %q", gotPath)
		}
		cb := gotBody["callback"].(map[string]any)
		if cb["event"] != "invoice.create" {
			t.Fatalf("body = %v", gotBody)
		}
		if got.CallbackID != 2001 || got.Verified {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Callbacks.Register(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[edge] the id field is accepted alongside callbackid", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "callbacks", "id_field"))
		got, err := c.Callbacks.Register(ctx, "ACM123", &CallbackRegisterRequest{Event: "estimate.create", URI: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if got.CallbackID != 2002 {
			t.Fatalf("got = %+v", got)
		}
	})
}

func TestCallbacksList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] returns subscribed callbacks", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "callbacks", "list"))
		page, err := c.Callbacks.List(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].Event != "invoice.create" {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[edge] no subscriptions yet", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "callbacks", "list_empty"))
		page, err := c.Callbacks.List(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 0 || page.Total != 0 {
			t.Fatalf("page = %+v", page)
		}
	})
}

func TestCallbacksDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] deletes by callback id", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			_, _ = io.WriteString(w, `{"response": {}}`)
		}))
		if err := c.Callbacks.Delete(ctx, "ACM123", 2001); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/events/account/ACM123/events/callbacks/2001" || gotMethod != http.MethodDelete {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
	})

	t.Run("[sad] deleting an unknown callback", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if err := c.Callbacks.Delete(ctx, "ACM123", 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestCallbacksVerify(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the verifier code", func(t *testing.T) {
		var gotBody map[string]any
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "callbacks", "verify")(w, r)
		}))
		got, err := c.Callbacks.Verify(ctx, "ACM123", 2001, "the-verifier-code")
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q", gotMethod)
		}
		cb := gotBody["callback"].(map[string]any)
		if cb["verifier"] != "the-verifier-code" {
			t.Fatalf("body = %v", gotBody)
		}
		if !got.Verified {
			t.Fatalf("got = %+v", got)
		}
	})
}

func TestCallbacksResendVerification(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] shares Verify's PUT path but sends a distinct resend body", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			_, _ = io.WriteString(w, `{"response": {"result": {"callback": {"callbackid": 2001, "verified": false}}}}`)
		}))
		if err := c.Callbacks.ResendVerification(ctx, "ACM123", 2001); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/events/account/ACM123/events/callbacks/2001" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		cb := gotBody["callback"].(map[string]any)
		if cb["resend"] != true {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] resending for an unknown callback", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if err := c.Callbacks.ResendVerification(ctx, "ACM123", 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}
