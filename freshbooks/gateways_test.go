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

	t.Run("[parity] decodes the live stripe_unified connection", func(t *testing.T) {
		// Phase 7 (live, 2026-09-02): an account onboarded through
		// FreshBooks Payments answers "stripe": null, no "fbpay" key at
		// all, and the whole connection under "stripe_unified".
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveFixture(t, http.StatusOK, "gateways", "get_stripe_unified")(w, r)
		}))
		got, err := c.Gateways.Get(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d connections", len(got))
		}
		conn := got[0]
		if conn.FBPay != nil || conn.Stripe != nil {
			t.Fatalf("fbpay/stripe should be absent: %+v", conn)
		}
		su := conn.StripeUnified
		if su == nil {
			t.Fatal("stripe_unified did not decode")
		}
		if su.StripeAccountID == "" || su.PublishableKey == "" || su.StripeEmail == "" {
			t.Fatalf("stripe_unified = %+v", su)
		}
		if !su.StripeChargesEnabled || !su.StripePayoutsEnabled {
			t.Fatalf("charges/payouts flags = %+v", su)
		}
		if su.AccountStatus != "fully_enabled" || su.OnboardingCompletionPercent != 100 {
			t.Fatalf("status = %q, percent = %d", su.AccountStatus, su.OnboardingCompletionPercent)
		}
		if su.StripePayoutsScheduleInterval != "daily" || su.StripePayoutsScheduleDelayDays != 7 {
			t.Fatalf("payout schedule = %+v", su)
		}
		if len(su.AvailableOnboardingStrategies) == 0 || su.AvailableOnboardingStrategies[0] != "hosted" {
			t.Fatalf("onboarding strategies = %v", su.AvailableOnboardingStrategies)
		}
		if len(su.Capabilities) == 0 || su.Capabilities[0].Capability == "" {
			t.Fatalf("capabilities = %+v", su.Capabilities)
		}
		if su.ChargesFirstEnabledAt.IsZero() || su.StripeAccountUpdatedAt.IsZero() {
			t.Fatalf("timestamps = %+v", su)
		}
		if string(su.StripeRequirementsCurrentDeadline) != "null" {
			t.Fatalf("deadline = %s", su.StripeRequirementsCurrentDeadline)
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
