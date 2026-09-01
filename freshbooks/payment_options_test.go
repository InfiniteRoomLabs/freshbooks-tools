package freshbooks

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// newHostOverrideClient returns a client whose base URL is a host distinct
// from tokenizationHost, with a transport that dials srv regardless of
// which host a request's URL names. This proves FBPayTokenize/
// StripeTokenize actually override the destination host rather than
// falling back to the client's own base host.
func newHostOverrideClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, srv.Listener.Addr().String())
		},
	}
	c, err := NewClient(
		WithBaseURL("http://api.freshbooks.test"),
		WithHTTPClient(&http.Client{Transport: tr}),
		WithTokenSource(auth.StaticTokenSource("test-access-token")),
	)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPaymentOptionsFBPayTokenize(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts to the tokenization host, not the API base URL", func(t *testing.T) {
		var gotHost, gotPath string
		var gotBody map[string]any
		tokenizeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotHost, gotPath = r.Host, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusCreated, "payment_options", "fbpay_tokenize")(w, r)
		}))
		defer tokenizeSrv.Close()

		c := newHostOverrideClient(t, tokenizeSrv)
		got, err := c.PaymentOptions.FBPayTokenize(ctx, &FBPayTokenizeRequest{
			Name: "John Johnston", CardNumber: "4500123456789012", ExpiryMonth: "10", ExpiryYear: "2024",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/gateway/fbpay/tokenize" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotHost != tokenizationHost {
			t.Fatalf("host = %q, want %q", gotHost, tokenizationHost)
		}
		ccInfo := gotBody["cc_info"].(map[string]any)
		if ccInfo["card_number"] != "4500123456789012" {
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
		var gotBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			_, _ = io.WriteString(w, `{"id": "pm_example", "object": "payment_method"}`)
		}))
		defer srv.Close()

		c := newHostOverrideClient(t, srv)
		raw, err := c.PaymentOptions.StripeTokenize(ctx, &StripeTokenizeRequest{
			Name: "Mush Parker", CardNumber: "450001234567809012", APIKey: "pk_live_example",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotBody["api_key"] != "pk_live_example" {
			t.Fatalf("body = %v", gotBody)
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
}
