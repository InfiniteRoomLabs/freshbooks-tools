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

func TestIdentityAddBusiness(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the business payload", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "settings", "business_added")(w, r)
		}))

		b, err := c.Identity.AddBusiness(ctx, &BusinessCreateRequest{Name: "Example Business LLC", DateFormat: "mm/dd/yyyy"})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/auth/api/v1/users/business" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if gotBody["name"] != "Example Business LLC" {
			t.Fatalf("body = %v", gotBody)
		}
		if b.ID != 1001 || b.Address.Country != "Canada" {
			t.Fatalf("business = %+v", b)
		}
		if len(b.BusinessGroup.Members) != 1 || b.BusinessGroup.Members[0].IdentityID != 4242424 {
			t.Fatalf("business_group = %+v", b.BusinessGroup)
		}
		if b.PhoneNumber != nil {
			t.Fatalf("a captured-null phone_number should decode to nil: %+v", b)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Identity.AddBusiness(ctx, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Identity.AddBusiness(ctx, &BusinessCreateRequest{Name: "x"}); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestIdentityDeleteBusiness(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a real DELETE", func(t *testing.T) {
		var gotMethod, gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		if err := c.Identity.DeleteBusiness(ctx, BusinessID(8675309)); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodDelete || gotPath != "/auth/api/v1/users/business/8675309" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
	})

	t.Run("[sad] a 422 while a subscription is still active", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "settings", "error_422_subscription_active"))
		err := c.Identity.DeleteBusiness(ctx, BusinessID(1))
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
		if err.Error() == "" {
			t.Fatal("want a message")
		}
	})
}

func TestIdentityDeleteBusinessSubscription(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] DELETEs the rewritten public path", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "settings", "subscription_deleted")(w, r)
		}))
		if err := c.Identity.DeleteBusinessSubscription(ctx, AccountID("ACM123")); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/auth/api/v1/billing/account/ACM123/subscription" {
			t.Fatalf("path = %q", gotPath)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if err := c.Identity.DeleteBusinessSubscription(ctx, AccountID("ACM123")); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] a hostile AccountID never reaches the network", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if err := c.Identity.DeleteBusinessSubscription(ctx, AccountID("../x")); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestIdentityProvisionPayments(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the enrollment payload, a null body is not an error", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "null")
		}))
		err := c.Identity.ProvisionPayments(ctx, AccountID("ACM123"), &PaymentsProvisionRequest{
			FirstName: "Sam", LastName: "Owner", Email: "owner@example.com", OrgName: "Example Business LLC",
			RedirectBaseURI: "https://app.example.com/gatewayConnectRedirect/fbpay/", Country: "US",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/payments/account/ACM123/gateway/fbpay" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if err := c.Identity.ProvisionPayments(ctx, AccountID("ACM123"), nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a hostile AccountID never reaches the network", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		err := c.Identity.ProvisionPayments(ctx, AccountID("a/b"), &PaymentsProvisionRequest{})
		if err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestIdentityCreateApplication(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the application payload", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "settings", "application_created")(w, r)
		}))
		app, err := c.Identity.CreateApplication(ctx, &ApplicationCreateRequest{
			Name: "Example App", RedirectURI: "https://app.example.com/callback",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotBody["name"] != "Example App" {
			t.Fatalf("body = %v", gotBody)
		}
		if app.ClientID != "syn-client-abc123" {
			t.Fatalf("app = %+v", app)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Identity.CreateApplication(ctx, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Identity.CreateApplication(ctx, &ApplicationCreateRequest{Name: "x"}); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestIdentityApplications(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] lists registered applications", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "settings", "applications_list")(w, r)
		}))
		apps, err := c.Identity.Applications(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/auth/api/v1/partners/applications" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(apps) != 1 || apps[0].ClientID != "syn-client-abc123" {
			t.Fatalf("apps = %+v", apps)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Identity.Applications(ctx); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestIdentityUpdateApplication(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts every field, no omitempty on the full-replace body", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "settings", "application_created")(w, r)
		}))
		app, err := c.Identity.UpdateApplication(ctx, "syn-client-abc123", &ApplicationUpdateRequest{
			Name: "Example App", ClientSecret: "syn-secret-def456", RedirectURI: "https://app.example.com/callback",
			Description: "",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut || gotPath != "/auth/api/v1/partners/applications/syn-client-abc123" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if _, ok := gotBody["description"]; !ok {
			t.Fatal("an explicit empty description must still be sent on a full-replace PUT")
		}
		if app.ClientID != "syn-client-abc123" {
			t.Fatalf("app = %+v", app)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Identity.UpdateApplication(ctx, "x", nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "auth", "error_401"))
		req := &ApplicationUpdateRequest{Name: "x", ClientSecret: "s", RedirectURI: "https://example.test"}
		if _, err := c.Identity.UpdateApplication(ctx, "x", req); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a hostile clientID never reaches the network", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		req := &ApplicationUpdateRequest{Name: "x", ClientSecret: "s", RedirectURI: "https://example.test"}
		if _, err := c.Identity.UpdateApplication(ctx, "../x", req); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestApplicationStringRedacted(t *testing.T) {
	t.Run("[happy] Application.String never prints the client secret", func(t *testing.T) {
		app := Application{ClientID: "syn-client-abc123", ClientSecret: "syn-secret-def456", Name: "Example App"}
		s := app.String()
		if strings.Contains(s, "syn-secret-def456") {
			t.Fatal("client secret leaked into String()")
		}
		if !strings.Contains(s, "redacted") {
			t.Fatalf("String() = %q, want it to say redacted", s)
		}
	})

	t.Run("[happy] ApplicationUpdateRequest.String never prints the client secret", func(t *testing.T) {
		req := ApplicationUpdateRequest{Name: "Example App", ClientSecret: "syn-secret-def456"}
		s := req.String()
		if strings.Contains(s, "syn-secret-def456") {
			t.Fatal("client secret leaked into String()")
		}
		if !strings.Contains(s, "redacted") {
			t.Fatalf("String() = %q, want it to say redacted", s)
		}
	})
}
