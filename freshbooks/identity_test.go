package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestIdentityMe(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] maps business_memberships onto every ID type", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "auth", "users_me")(w, r)
		}))

		memberships, err := c.Identity.Me(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/auth/api/v1/users/me" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(memberships) != 1 {
			t.Fatalf("got %d memberships", len(memberships))
		}
		m := memberships[0]
		if m.AccountID != "ACM123" {
			t.Errorf("AccountID = %q", m.AccountID)
		}
		if m.BusinessID != 8675309 {
			t.Errorf("BusinessID = %d", m.BusinessID)
		}
		if m.BusinessUUID != "00000000-0000-4000-8000-000000000001" {
			t.Errorf("BusinessUUID = %q", m.BusinessUUID)
		}
		if m.Name != "Example Business LLC" || m.Role != "owner" {
			t.Errorf("membership = %+v", m)
		}
	})

	t.Run("[happy] Whoami carries the identity fields too", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "auth", "users_me"))
		u, err := c.Identity.Whoami(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if u.ID != 4242424 || u.Email != "owner@example.com" || u.FirstName != "Sam" || u.LastName != "Owner" {
			t.Fatalf("user = %+v", u)
		}
		if len(u.Memberships) != 1 {
			t.Fatalf("memberships = %d", len(u.Memberships))
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Identity.Me(ctx); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[edge] an identity with no memberships", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"response": {"id": 1, "business_memberships": []}}`)
		}))
		memberships, err := c.Identity.Me(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(memberships) != 0 {
			t.Fatalf("got %d memberships", len(memberships))
		}
	})
}

func TestIdentityRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the registration payload", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			_, _ = io.WriteString(w, `{"response": {"id": 99, "email": "new@example.com"}}`)
		}))

		u, err := c.Identity.Register(ctx, &RegisterRequest{
			ID:           "new@example.com",
			Email:        "new@example.com",
			Password:     "a-synthetic-password",
			Country:      "United States",
			CurrencyCode: "USD",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/auth/api/v1/smux/registrations" || gotMethod != http.MethodPost {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if gotBody["email"] != "new@example.com" || gotBody["currencyCode"] != "USD" {
			t.Fatalf("body = %v", gotBody)
		}
		if _, ok := gotBody["company_name"]; ok {
			t.Fatal("an unset optional should be omitted")
		}
		if u.ID != 99 || u.Email != "new@example.com" {
			t.Fatalf("user = %+v", u)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Identity.Register(ctx, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a rejected registration never echoes the password", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"error": "invalid_email", "error_description": "bad address"}`)
		}))
		_, err := c.Identity.Register(ctx, &RegisterRequest{Email: "x", Password: "a-synthetic-password"})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
		if strings.Contains(err.Error(), "a-synthetic-password") {
			t.Fatal("the password reached an error string")
		}
	})
}
