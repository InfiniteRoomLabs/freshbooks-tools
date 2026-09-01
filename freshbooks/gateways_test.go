package freshbooks

import (
	"context"
	"net/http"
	"testing"
)

func TestGatewaysGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes fbpay and stripe connections, flat with no envelope", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "gateways", "get")(w, r)
		}))
		got, err := c.Gateways.Get(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/payments/account/ACM123/gateway" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(got) != 1 {
			t.Fatalf("got %d connections", len(got))
		}
		conn := got[0]
		if conn.FBPay == nil || conn.FBPay.State != "active" || conn.FBPay.Pricing.PercentNonAmex != "2.90" {
			t.Fatalf("fbpay = %+v", conn.FBPay)
		}
		if conn.Stripe == nil || conn.Stripe.PublishableKey != "pk_test_example" {
			t.Fatalf("stripe = %+v", conn.Stripe)
		}
		if conn.FBPay.BankInfo.BankName == "" || conn.FBPay.BankInfo.LastPaymentAmount != "1240.79" {
			t.Fatalf("bank_info = %+v", conn.FBPay.BankInfo)
		}
		if conn.Stripe.MaxACHFee != 5000 {
			t.Fatalf("MaxACHFee = %d, want the bare-number capture", conn.Stripe.MaxACHFee)
		}
		if conn.FBPay.ActionReasons == nil {
			t.Fatalf("ActionReasons did not decode: %+v", conn.FBPay)
		}
	})

	t.Run("[edge] no gateway connected yet", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"gateway_connections": []}`))
		}))
		got, err := c.Gateways.Get(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a 403 for an account without payments enabled", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error": "forbidden"}`))
		}))
		if _, err := c.Gateways.Get(ctx, "ACM123"); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] an unsafe account id", func(t *testing.T) {
		called := false
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		if _, err := c.Gateways.Get(ctx, "a/b"); err == nil {
			t.Fatal("want an error")
		}
		if called {
			t.Fatal("a request was made with an unsafe account id")
		}
	})
}
