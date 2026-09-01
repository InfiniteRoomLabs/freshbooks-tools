package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestInvoiceProfilesList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes the page", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "invoice_profiles", "list")(w, r)
		}))
		page, err := c.InvoiceProfiles.List(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/invoice_profiles/invoice_profiles" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(page.Items) != 1 || page.Items[0].ProfileID != 700 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.InvoiceProfiles.List(ctx, "ACM123", nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestInvoiceProfilesAll(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] iterates every profile once", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "invoice_profiles", "list"))
		var got []InvoiceProfile
		for p, err := range c.InvoiceProfiles.All(ctx, "ACM123", nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, p)
		}
		if len(got) != 1 {
			t.Fatalf("got %d profiles", len(got))
		}
	})
}

func TestInvoiceProfilesGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] fetches one profile", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "invoice_profiles", "profile")(w, r)
		}))
		p, err := c.InvoiceProfiles.Get(ctx, "ACM123", 700)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/invoice_profiles/invoice_profiles/700" {
			t.Fatalf("path = %q", gotPath)
		}
		if p.CustomerID != 55001 || p.Frequency != "m" {
			t.Fatalf("profile = %+v", p)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.InvoiceProfiles.Get(ctx, "ACM123", 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestInvoiceProfilesCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a plain template and a time-entry-holder template share one call", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "invoice_profiles", "profile")(w, r)
		}))
		req := &InvoiceProfileCreateRequest{
			CustomerID:          55001,
			Frequency:           "m",
			NumberRecurring:     3,
			IncludeUnbilledTime: boolPtr(true),
			ProjectFormat:       &ProjectFormat{GroupBy: "service", Method: "detailed"},
		}
		p, err := c.InvoiceProfiles.Create(ctx, "ACM123", req)
		if err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["invoice_profile"].(map[string]any)
		if envelope["frequency"] != "m" {
			t.Fatalf("body = %v", gotBody)
		}
		if _, ok := envelope["create_date"]; ok {
			t.Fatalf("body = %v, want an unset CreateDate omitted rather than sent as null", gotBody)
		}
		pf := envelope["project_format"].(map[string]any)
		if pf["group_by"] != "service" {
			t.Fatalf("project_format = %v", pf)
		}
		if p.ProfileID != 700 {
			t.Fatalf("profile = %+v", p)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.InvoiceProfiles.Create(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] an invalid create_date is a validation error", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if _, err := c.InvoiceProfiles.Create(ctx, "ACM123", &InvoiceProfileCreateRequest{CustomerID: 1, Frequency: "m"}); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestInvoiceProfilesUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "invoice_profiles", "profile")(w, r)
		}))
		freq := "2w"
		n := 12
		if _, err := c.InvoiceProfiles.Update(ctx, "ACM123", 700, &InvoiceProfileUpdateRequest{Frequency: &freq, NumberRecurring: &n}); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %s", gotMethod)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.InvoiceProfiles.Update(ctx, "ACM123", 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestInvoiceProfilesDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts vis_state 1", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "invoice_profiles", "profile")(w, r)
		}))
		if err := c.InvoiceProfiles.Delete(ctx, "ACM123", 700); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["invoice_profile"].(map[string]any)
		if envelope["vis_state"] != float64(1) {
			t.Fatalf("body = %v", gotBody)
		}
	})
}

func TestInvoiceProfilesEnablePaymentOptions(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the gateway payload", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"payment_options": {"gateway_name": "stripe"}}`))
		}))
		req := &PaymentOptionsRequest{HasCreditCard: true, HasACHTransfer: true, GatewayName: "stripe"}
		if err := c.InvoiceProfiles.EnablePaymentOptions(ctx, "ACM123", 700, req); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/payments/account/ACM123/invoice_profile/700/payment_options" {
			t.Fatalf("path = %q", gotPath)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if err := c.InvoiceProfiles.EnablePaymentOptions(ctx, "ACM123", 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func boolPtr(b bool) *bool { return &b }
