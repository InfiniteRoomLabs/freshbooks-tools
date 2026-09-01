package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestInvoicesList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes the page and pagination", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.String()
			serveFixture(t, http.StatusOK, "invoices", "list")(w, r)
		}))

		page, err := c.Invoices.List(ctx, "ACM123", &InvoiceListOptions{Search: Search{"status": "1"}, Page: 1, PerPage: 15})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/invoices/invoices?page=1&per_page=15&search%5Bstatus%5D=1" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(page.Items) != 2 || page.Total != 2 || page.Page != 1 {
			t.Fatalf("page = %+v", page)
		}
		if page.Items[0].InvoiceNumber != "0000001" || page.Items[0].Amount.Amount != "180.00" {
			t.Fatalf("first invoice = %+v", page.Items[0])
		}
	})

	t.Run("[edge] a nil options pointer lists with no filters", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "invoices", "list")(w, r)
		}))
		if _, err := c.Invoices.List(ctx, "ACM123", nil); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "" {
			t.Fatalf("query = %q, want empty", gotQuery)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Invoices.List(ctx, "ACM123", nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestInvoicesAll(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] iterates every invoice once", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "invoices", "list"))
		var got []Invoice
		for inv, err := range c.Invoices.All(ctx, "ACM123", nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, inv)
		}
		if len(got) != 2 {
			t.Fatalf("got %d invoices", len(got))
		}
	})
}

func TestInvoicesGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] fetches one invoice", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "invoices", "invoice")(w, r)
		}))
		inv, err := c.Invoices.Get(ctx, "ACM123", 90001)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/invoices/invoices/90001" {
			t.Fatalf("path = %q", gotPath)
		}
		if inv.InvoiceID != 90001 || inv.CustomerID != 55001 {
			t.Fatalf("invoice = %+v", inv)
		}
		if inv.UUID == "" || inv.Version == "" {
			t.Fatalf("invoice = %+v, want UUID and Version decoded", inv)
		}
		if inv.SentID != 1 || inv.BasecampID != 0 || inv.ExtArchive != 0 || inv.GMail != false {
			t.Fatalf("invoice = %+v", inv)
		}
		if inv.DepositStatus != "none" {
			t.Fatalf("invoice = %+v", inv)
		}
		if inv.AutobillStatus != nil || inv.DisputeStatus != nil || inv.DepositPercentage != nil || inv.LastOrderStatus != nil || inv.NetPaidAmount != nil {
			t.Fatalf("invoice = %+v, want the null docs-only fields to decode as nil pointers", inv)
		}
	})

	t.Run("[happy] Include(\"presentation\") is the same call as the w/Logo request", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "invoices", "invoice")(w, r)
		}))
		if _, err := c.Invoices.Get(ctx, "ACM123", 90001, Include("presentation")); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "include%5B%5D=presentation" {
			t.Fatalf("query = %q", gotQuery)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Invoices.Get(ctx, "ACM123", 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestInvoicesCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a customerid+lines invoice", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "invoices", "invoice")(w, r)
		}))

		req := &InvoiceCreateRequest{
			CustomerID: 55001,
			Lines: []InvoiceLine{
				{Type: 0, Name: "Paperwork", Qty: 1, UnitCost: Money{Amount: "5000.00", Code: "USD"}},
			},
		}
		inv, err := c.Invoices.Create(ctx, "ACM123", req)
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/accounting/account/ACM123/invoices/invoices" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		envelope, ok := gotBody["invoice"].(map[string]any)
		if !ok || envelope["customerid"] != float64(55001) {
			t.Fatalf("body = %v", gotBody)
		}
		if _, ok := envelope["create_date"]; ok {
			t.Fatalf("body = %v, want an unset CreateDate omitted rather than sent as null", gotBody)
		}
		line := envelope["lines"].([]any)[0].(map[string]any)
		if _, ok := line["amount"]; ok {
			t.Fatalf("line = %v, want the server-filled Amount omitted on write", line)
		}
		if inv.InvoiceID != 90001 {
			t.Fatalf("invoice = %+v", inv)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Invoices.Create(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a validation error is ErrValidation", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if _, err := c.Invoices.Create(ctx, "ACM123", &InvoiceCreateRequest{CustomerID: 1}); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestInvoicesUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the changed fields, including gateway toggling", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "invoices", "invoice")(w, r)
		}))

		status := 2
		req := &InvoiceUpdateRequest{Status: &status, AllowedGatewayIDs: []int64{30}}
		if _, err := c.Invoices.Update(ctx, "ACM123", 90001, req); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %s", gotMethod)
		}
		envelope := gotBody["invoice"].(map[string]any)
		if envelope["status"] != float64(2) {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Invoices.Update(ctx, "ACM123", 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestInvoicesDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts vis_state 1", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "invoices", "invoice")(w, r)
		}))
		if err := c.Invoices.Delete(ctx, "ACM123", 90001); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %s", gotMethod)
		}
		envelope := gotBody["invoice"].(map[string]any)
		if envelope["vis_state"] != float64(1) {
			t.Fatalf("body = %v", gotBody)
		}
	})
}

func TestInvoicesSend(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] sends action_email with recipients and a custom subject", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"response": {}}`))
		}))
		req := &InvoiceSendRequest{EmailRecipients: []string{"client@example.test"}, Subject: "Your invoice"}
		if err := c.Invoices.Send(ctx, "ACM123", 90001, req); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["invoice"].(map[string]any)
		if envelope["action_email"] != true {
			t.Fatalf("body = %v", gotBody)
		}
		recipients := envelope["email_recipients"].([]any)
		if len(recipients) != 1 || recipients[0] != "client@example.test" {
			t.Fatalf("recipients = %v", recipients)
		}
		email := envelope["invoice_customized_email"].(map[string]any)
		if email["subject"] != "Your invoice" {
			t.Fatalf("email = %v", email)
		}
	})

	t.Run("[edge] a nil request still marks the invoice sent", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"response": {}}`))
		}))
		if err := c.Invoices.Send(ctx, "ACM123", 90001, nil); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["invoice"].(map[string]any)
		if envelope["action_email"] != true {
			t.Fatalf("body = %v", gotBody)
		}
		if _, ok := envelope["invoice_customized_email"]; ok {
			t.Fatal("an unset customization should be omitted")
		}
	})
}

func TestInvoicesPDF(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] returns the raw, non-JSON body and asks for a PDF", func(t *testing.T) {
		var gotPath, gotAccept string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAccept = r.Header.Get("Accept")
			w.Header().Set("Content-Type", "application/pdf")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("%PDF-1.4 fake pdf bytes"))
		}))
		raw, err := c.Invoices.PDF(ctx, "ACM123", 90001)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/invoices/invoices/90001/pdf" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotAccept != "application/pdf" {
			t.Fatalf("Accept = %q, want application/pdf", gotAccept)
		}
		if string(raw) != "%PDF-1.4 fake pdf bytes" {
			t.Fatalf("raw = %q", raw)
		}
	})

	t.Run("[sad] a non-2xx status decodes as an API error, not raw bytes", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Invoices.PDF(ctx, "ACM123", 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] a 200 whose body is not a PDF is rejected", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>please sign in</html>"))
		}))
		if _, err := c.Invoices.PDF(ctx, "ACM123", 1); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[happy] a 429 with Retry-After is retried, proving PDF shares do's retry loop", func(t *testing.T) {
		var hits atomic.Int32
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if hits.Add(1) == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error": "rate limited"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("%PDF-1.4 fake pdf bytes"))
		}), WithRetry(testRetry(2)))

		raw, err := c.Invoices.PDF(ctx, "ACM123", 90001)
		if err != nil {
			t.Fatal(err)
		}
		if hits.Load() != 2 {
			t.Fatalf("hits = %d, want 2 attempts", hits.Load())
		}
		if string(raw) != "%PDF-1.4 fake pdf bytes" {
			t.Fatalf("raw = %q", raw)
		}
	})
}

func TestInvoicesShareLink(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] both Share Link and Share PDF resolve here", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.String()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"response": {"result": {"share_link": {
				"clientid": 55001, "resource_type": "invoice", "resourceid": "90001",
				"share_link": "https://my.freshbooks.com/#/link/example", "share_method": "share_link"
			}}}}`))
		}))
		link, err := c.Invoices.ShareLink(ctx, "ACM123", 90001)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/invoices/invoices/90001/share_link?share_method=share_link" {
			t.Fatalf("path = %q", gotPath)
		}
		if link.ClientID != 55001 || link.ShareMethod != "share_link" {
			t.Fatalf("link = %+v", link)
		}
	})

	t.Run("[sad] an unsent invoice is a 422", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if _, err := c.Invoices.ShareLink(ctx, "ACM123", 1); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestInvoicesEnablePaymentOptions(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the gateway payload", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"payment_options": {"gateway_name": "fbpay"}}`))
		}))
		req := &PaymentOptionsRequest{HasCreditCard: true, HasACHTransfer: true, AllowPartialPayments: true, GatewayName: "fbpay"}
		if err := c.Invoices.EnablePaymentOptions(ctx, "ACM123", 90001, req); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/payments/account/ACM123/invoice/90001/payment_options" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotBody["gateway_name"] != "fbpay" || gotBody["has_credit_card"] != true {
			t.Fatalf("body = %v", gotBody)
		}
		if gotBody["entity_type"] != "invoice" || gotBody["entity_id"] != float64(90001) {
			t.Fatalf("body = %v, want entity_type \"invoice\" and entity_id 90001", gotBody)
		}
	})

	t.Run("[happy] a false toggle survives into the body, not dropped by omitempty", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"payment_options": {}}`))
		}))
		req := &PaymentOptionsRequest{GatewayName: "fbpay", HasCreditCard: false, HasACHTransfer: true}
		if err := c.Invoices.EnablePaymentOptions(ctx, "ACM123", 90001, req); err != nil {
			t.Fatal(err)
		}
		if v, ok := gotBody["has_credit_card"]; !ok || v != false {
			t.Fatalf("body = %v, want has_credit_card explicitly false", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if err := c.Invoices.EnablePaymentOptions(ctx, "ACM123", 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestInvoicesRejectUnsafeAccountID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		acct AccountID
	}{
		{"[sad] a query delimiter", "ACM123?per_page=100"},
		{"[sad] a path separator", "ACM123/other"},
		{"[sad] a directory traversal", "ACM123/../.."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			if _, err := c.Invoices.Get(ctx, tc.acct, 1); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}

func TestInvoicePresentationDefaults(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes the account's default presentation", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"response": {"result": {"presentation": {
				"date_format": "mm/dd/yyyy", "theme_font_name": "modern", "theme_layout": "simple",
				"theme_primary_color": "#663399"
			}}}}`))
		}))
		p, err := c.Invoices.InvoicePresentationDefaults(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/invoices/presentations" {
			t.Fatalf("path = %q", gotPath)
		}
		if p.ThemePrimaryColor != "#663399" {
			t.Fatalf("presentation = %+v", p)
		}
	})
}
