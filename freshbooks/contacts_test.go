package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestContactsUpdate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts the contact payload", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "contacts_update")(w, r)
		}))
		contact, err := c.Contacts.Update(ctx, acct, 900001, &ContactUpdateRequest{
			FirstName: "Secondary",
			LastName:  "Contact",
			Email:     "secondary@example.com",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/users/contacts/900001" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["contact"].(map[string]any)
		if inner["email"] != "secondary@example.com" {
			t.Fatalf("body = %v", gotBody)
		}
		if contact.UserID != 900001 {
			t.Fatalf("contact = %+v", contact)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Contacts.Update(ctx, acct, 900001, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Contacts.Update(ctx, acct, 999, &ContactUpdateRequest{FirstName: "x"}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestContactsDelete(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] issues a real DELETE", func(t *testing.T) {
		var gotMethod, gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"response": {}}`))
		}))
		if err := c.Contacts.Delete(ctx, acct, 900001); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodDelete {
			t.Fatalf("method = %q, want DELETE", gotMethod)
		}
		if gotPath != "/accounting/account/ACM123/users/contacts/900001" {
			t.Fatalf("path = %q", gotPath)
		}
	})

	t.Run("[sad] a 403 is ErrForbidden", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"response": {"errors": [{"errno": 1000, "message": "forbidden"}]}}`))
		}))
		if err := c.Contacts.Delete(ctx, acct, 900001); !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v", err)
		}
	})
}
