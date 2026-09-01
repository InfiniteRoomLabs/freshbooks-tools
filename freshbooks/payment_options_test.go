package freshbooks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPaymentOptionsFBPayTokenize(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts to the tokenization host over https, not the API base URL", func(t *testing.T) {
		rt := &recordingRoundTripper{resp: `{"cc_token": "1122334455"}`, status: http.StatusCreated}
		c := newRecordingClient(t, "http://api.freshbooks.test", rt)

		got, err := c.PaymentOptions.FBPayTokenize(ctx, &FBPayTokenizeRequest{
			Name: "John Johnston", CardNumber: "4111111111111111", ExpiryMonth: "10", ExpiryYear: "2024",
		})
		if err != nil {
			t.Fatal(err)
		}
		if rt.req.URL.Path != "/gateway/fbpay/tokenize" {
			t.Fatalf("path = %q", rt.req.URL.Path)
		}
		if rt.req.URL.Host != tokenizationHost || rt.req.URL.Scheme != "https" {
			t.Fatalf("url = %q, want https://%s", rt.req.URL, tokenizationHost)
		}
		var gotBody map[string]any
		_ = json.NewDecoder(rt.req.Body).Decode(&gotBody)
		ccInfo := gotBody["cc_info"].(map[string]any)
		if ccInfo["card_number"] != "4111111111111111" {
			t.Fatalf("body = %v", gotBody)
		}
		if got != "1122334455" {
			t.Fatalf("got = %q", got)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.PaymentOptions.FBPayTokenize(ctx, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestPaymentOptionsStripeTokenize(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts api_key alongside cc_info and returns raw JSON", func(t *testing.T) {
		rt := &recordingRoundTripper{resp: `{"id": "pm_example", "object": "payment_method"}`}
		c := newRecordingClient(t, "http://api.freshbooks.test", rt)

		raw, err := c.PaymentOptions.StripeTokenize(ctx, &StripeTokenizeRequest{
			Name: "Mush Parker", CardNumber: "4242424242424242", APIKey: "pk_test_example",
		})
		if err != nil {
			t.Fatal(err)
		}
		var gotBody map[string]any
		_ = json.NewDecoder(rt.req.Body).Decode(&gotBody)
		if gotBody["api_key"] != "pk_test_example" {
			t.Fatalf("body = %v", gotBody)
		}
		if _, dup := gotBody["cc_info"].(map[string]any)["api_key"]; dup {
			t.Fatal("api_key was sent twice: once at the top level and once inside cc_info")
		}
		if !strings.Contains(string(raw), "pm_example") {
			t.Fatalf("raw = %s", raw)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.PaymentOptions.StripeTokenize(ctx, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestPaymentOptionsStripeCreateSetupIntent(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the payment_method to the api.freshbooks.com host", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			_, _ = io.WriteString(w, `{"token": "stripe_setup_token"}`)
		}))
		raw, err := c.PaymentOptions.StripeCreateSetupIntent(ctx, "ACM123", "pm_d87sh438shuiasf4289437u")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/payments/account/ACM123/gateway/stripe/credit-card/token" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotBody["payment_method"] != "pm_d87sh438shuiasf4289437u" {
			t.Fatalf("body = %v", gotBody)
		}
		if !strings.Contains(string(raw), "stripe_setup_token") {
			t.Fatalf("raw = %s", raw)
		}
	})

	t.Run("[sad] an unsafe account id", func(t *testing.T) {
		called := false
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		if _, err := c.PaymentOptions.StripeCreateSetupIntent(ctx, "a/b", "pm_x"); err == nil {
			t.Fatal("want an error")
		}
		if called {
			t.Fatal("a request was made with an unsafe account id")
		}
	})
}

func TestPaymentOptionsSaveCreditCard(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a nested credit_card body shared by both gateways", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusCreated, "payment_options", "save_credit_card")(w, r)
		}))
		got, err := c.PaymentOptions.SaveCreditCard(ctx, "ACM123", &SaveCreditCardRequest{
			CardHolderUserID: 123456,
			Access:           CreditCardAccess{System: true, InvoiceProfiles: []int64{121212}},
			CreditCardTokens: []CreditCardToken{{Token: "token_123", GatewayName: "fbpay", IsPrimary: true}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/payments/account/ACM123/credit-card" {
			t.Fatalf("path = %q", gotPath)
		}
		cc := gotBody["credit_card"].(map[string]any)
		if cc["card_holder_user_id"].(float64) != 123456 {
			t.Fatalf("body = %v", gotBody)
		}
		if got.CardType != "visa" || got.LastFourDigits != "0234" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.PaymentOptions.SaveCreditCard(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] an unsafe account id", func(t *testing.T) {
		called := false
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		if _, err := c.PaymentOptions.SaveCreditCard(ctx, "a/b", &SaveCreditCardRequest{}); err == nil {
			t.Fatal("want an error")
		}
		if called {
			t.Fatal("a request was made with an unsafe account id")
		}
	})
}

func TestTokenizeRequestStringRedacted(t *testing.T) {
	t.Run("[happy] FBPayTokenizeRequest.String never prints the card number or CVV", func(t *testing.T) {
		req := FBPayTokenizeRequest{Name: "Sam Owner", CardNumber: "4111111111111111", CVV: "123"}
		s := req.String()
		if strings.Contains(s, "4111111111111111") || strings.Contains(s, "123") {
			t.Fatalf("card data leaked into String(): %s", s)
		}
		if !strings.Contains(s, "redacted") {
			t.Fatalf("String() = %q, want it to say redacted", s)
		}
	})

	t.Run("[happy] StripeTokenizeRequest.String never prints the card number or API key", func(t *testing.T) {
		req := StripeTokenizeRequest{Name: "Sam Owner", CardNumber: "4242424242424242", APIKey: "pk_test_example"}
		s := req.String()
		if strings.Contains(s, "4242424242424242") || strings.Contains(s, "pk_test_example") {
			t.Fatalf("card data leaked into String(): %s", s)
		}
		if !strings.Contains(s, "redacted") {
			t.Fatalf("String() = %q, want it to say redacted", s)
		}
	})
}
