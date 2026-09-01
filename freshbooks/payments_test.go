package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestPaymentsList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes the page", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "payments", "list")(w, r)
		}))
		page, err := c.Payments.List(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/payments/payments" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(page.Items) != 2 || page.Total != 2 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Payments.List(ctx, "ACM123", nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPaymentsAll(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] iterates every payment once", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "payments", "list"))
		var got []Payment
		for p, err := range c.Payments.All(ctx, "ACM123", nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, p)
		}
		if len(got) != 2 {
			t.Fatalf("got %d payments", len(got))
		}
	})
}

func TestPaymentsGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] fetches one payment", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "payments", "payment")(w, r)
		}))
		p, err := c.Payments.Get(ctx, "ACM123", 5001)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/payments/payments/5001" {
			t.Fatalf("path = %q", gotPath)
		}
		if p.InvoiceID != 90001 || p.Amount.Amount != "10.00" {
			t.Fatalf("payment = %+v", p)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Payments.Get(ctx, "ACM123", 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPaymentsCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a new payment", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "payments", "payment")(w, r)
		}))
		req := &PaymentCreateRequest{InvoiceID: 90001, Amount: Money{Amount: "10.00", Code: "USD"}, Type: "Check", Note: "Paid in full"}
		p, err := c.Payments.Create(ctx, "ACM123", req)
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/accounting/account/ACM123/payments/payments" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		envelope := gotBody["payment"].(map[string]any)
		if envelope["invoiceid"] != float64(90001) {
			t.Fatalf("body = %v", gotBody)
		}
		if p.ID != 5001 {
			t.Fatalf("payment = %+v", p)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Payments.Create(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestPaymentsUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "payments", "payment")(w, r)
		}))
		note := "Updated with proper invoice amount"
		if _, err := c.Payments.Update(ctx, "ACM123", 5001, &PaymentUpdateRequest{Note: &note}); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %s", gotMethod)
		}
		envelope := gotBody["payment"].(map[string]any)
		if envelope["note"] != note {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Payments.Update(ctx, "ACM123", 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestPaymentsDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts vis_state 1", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "payments", "payment")(w, r)
		}))
		if err := c.Payments.Delete(ctx, "ACM123", 5001); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["payment"].(map[string]any)
		if envelope["vis_state"] != float64(1) {
			t.Fatalf("body = %v", gotBody)
		}
	})
}

func TestPaymentsCheckoutLinks(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] Create posts item_id, amount, and taxes", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"checkout_link": {"id": "cl_1", "item_id": "147778", "amount": "100", "is_active": true}}`))
		}))
		link := &CheckoutLink{ItemID: "147778", ItemName: "The Best Doorknob", Amount: "100", Currency: "CAD", IsActive: true, Taxes: []CheckoutLinkTax{{Name: "HST", Amount: 13}}}
		got, err := c.Payments.CreateCheckoutLink(ctx, "ACM123", link)
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/payments/account/ACM123/checkout-links" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if gotBody["item_id"] != "147778" {
			t.Fatalf("body = %v", gotBody)
		}
		if got.ID != "cl_1" {
			t.Fatalf("link = %+v", got)
		}
	})

	t.Run("[edge] Create falls back to the request when the response has no checkout_link key", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		link := &CheckoutLink{ItemID: "1", Amount: "10"}
		got, err := c.Payments.CreateCheckoutLink(ctx, "ACM123", link)
		if err != nil {
			t.Fatal(err)
		}
		if got != link {
			t.Fatalf("got = %+v, want the original request echoed back", got)
		}
	})

	t.Run("[sad] a nil checkout link", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Payments.CreateCheckoutLink(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[happy] Update puts to the link's id", func(t *testing.T) {
		var gotMethod, gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		link := &CheckoutLink{ItemID: "147778", Amount: "100", IsActive: false}
		if _, err := c.Payments.UpdateCheckoutLink(ctx, "ACM123", "cl_1", link); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut || gotPath != "/payments/account/ACM123/checkout-links/cl_1" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
	})

	t.Run("[sad] Update with a nil checkout link", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Payments.UpdateCheckoutLink(ctx, "ACM123", "cl_1", nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[happy] Delete issues a real HTTP DELETE", func(t *testing.T) {
		var gotMethod, gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		if err := c.Payments.DeleteCheckoutLink(ctx, "ACM123", "cl_1"); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodDelete || gotPath != "/payments/account/ACM123/checkout-links/cl_1" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
	})

	t.Run("[happy] UpdateCheckoutLinkGateway posts the gateway payload", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		req := &PaymentOptionsRequest{HasCreditCard: true, GatewayName: "stripe"}
		if err := c.Payments.UpdateCheckoutLinkGateway(ctx, "ACM123", "cl_1", req); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/payments/account/ACM123/checkout_link/cl_1/payment_options" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotBody["gateway_name"] != "stripe" {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] UpdateCheckoutLinkGateway with a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if err := c.Payments.UpdateCheckoutLinkGateway(ctx, "ACM123", "cl_1", nil); err == nil {
			t.Fatal("want an error")
		}
	})
}
