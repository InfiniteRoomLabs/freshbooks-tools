package freshbooks

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestReportsAccountsAging(t *testing.T) {
	ctx := context.Background()
	var gotPath string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		serveFixture(t, http.StatusOK, "reports", "accounts_aging")(w, r)
	}))

	t.Run("[happy] buckets a client's balance by age", func(t *testing.T) {
		got, err := c.Reports.AccountsAging(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/reports/accounting/accounts_aging" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(got.Accounts) != 1 || got.Accounts[0].Days0To30.Amount != "3830.00" {
			t.Fatalf("got = %+v", got.Accounts)
		}
		if got.Totals.Total.Amount != "4278.48" {
			t.Fatalf("totals = %+v", got.Totals)
		}
	})

	t.Run("[happy] date-range and currency options encode as bare query params", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "accounts_aging")(w, r)
		}))
		if _, err := c.Reports.AccountsAging(ctx, "ACM123", &AccountsAgingOptions{
			StartDate: "2019-01-01", EndDate: "2019-05-08", CurrencyCode: "USD",
		}); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "currency_code=USD&end_date=2019-05-08&start_date=2019-01-01" {
			t.Fatalf("query = %q", gotQuery)
		}
	})
}

func TestReportsBalanceSheet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] encodes repeated dates[] and decodes nested accounts", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "balance_sheet")(w, r)
		}))
		got, err := c.Reports.BalanceSheet(ctx, "ACM123", &BalanceSheetOptions{
			Dates: []string{"2019-04-19", "2019-04-25"}, CurrencyCode: "USD",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotQuery != "currency_code=USD&dates%5B%5D=2019-04-19&dates%5B%5D=2019-04-25" {
			t.Fatalf("query = %q", gotQuery)
		}
		if len(got.Data) != 2 || got.Data[0].Accounts[0].SubAccounts[0].SubAccountName != "Petty Cash" {
			t.Fatalf("got = %+v", got.Data)
		}
		if got.Data[1].Category != "" {
			t.Fatalf("null category should decode to empty string, got %q", got.Data[1].Category)
		}
	})

	t.Run("[edge] nil options send no query", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "balance_sheet")(w, r)
		}))
		if _, err := c.Reports.BalanceSheet(ctx, "ACM123", nil); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "" {
			t.Fatalf("query = %q", gotQuery)
		}
	})
}

func TestReportsUnknownShapeReports(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] BankReconciliationSummary returns raw JSON", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "bank_reconciliation_summary")(w, r)
		}))
		raw, err := c.Reports.BankReconciliationSummary(ctx, "ACM123", &BankReconciliationSummaryOptions{
			CurrencyCode: "USD", Date: "2019-04-24", BankAccountID: "42",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotQuery != "bank_accountid=42&currency_code=USD&date=2019-04-24" {
			t.Fatalf("query = %q", gotQuery)
		}
		if !strings.Contains(string(raw), "shape unknown") {
			t.Fatalf("raw = %s", raw)
		}
	})

	t.Run("[happy] ClientAccountStatement returns raw JSON", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "reports", "client_account_statement"))
		raw, err := c.Reports.ClientAccountStatement(ctx, "ACM123", &ClientAccountStatementOptions{StartDate: "2020-01-01"})
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) == 0 {
			t.Fatal("want a non-empty raw payload")
		}
	})

	t.Run("[happy] ExpenseDetails returns raw JSON", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "reports", "expense_details"))
		raw, err := c.Reports.ExpenseDetails(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw) == 0 {
			t.Fatal("want a non-empty raw payload")
		}
	})

	t.Run("[happy] ExpenseDetails date-range and currency options encode as bare query params", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "expense_details")(w, r)
		}))
		if _, err := c.Reports.ExpenseDetails(ctx, "ACM123", &ExpenseDetailsOptions{
			StartDate: "2019-01-01", EndDate: "2019-12-31", CurrencyCode: "USD",
		}); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "currency_code=USD&end_date=2019-12-31&start_date=2019-01-01" {
			t.Fatalf("query = %q", gotQuery)
		}
	})
}

func TestReportsDownloadInvoiceDetailsCSV(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] asks for text/csv and returns the bytes unparsed, bypassing envelope unwrap", func(t *testing.T) {
		var gotPath, gotAccept string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotAccept = r.URL.Path, r.Header.Get("Accept")
			w.Header().Set("Content-Type", "text/csv")
			_, _ = io.WriteString(w, "invoice_number,amount\n0000005,2800.00\n")
		}))
		got, err := c.Reports.DownloadInvoiceDetailsCSV(ctx, "ACM123", "tok_invoice_details")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/links/reports/tok_invoice_details/invoice_details.csv" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotAccept != "text/csv" {
			t.Fatalf("Accept = %q, want text/csv", gotAccept)
		}
		if string(got) != "invoice_number,amount\n0000005,2800.00\n" {
			t.Fatalf("got = %q", got)
		}
	})

	t.Run("[sad] an expired token", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Reports.DownloadInvoiceDetailsCSV(ctx, "ACM123", "expired"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] an unsafe account id or download token", func(t *testing.T) {
		called := false
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))
		if _, err := c.Reports.DownloadInvoiceDetailsCSV(ctx, "a/b", "tok"); err == nil {
			t.Fatal("want an error")
		}
		if _, err := c.Reports.DownloadInvoiceDetailsCSV(ctx, "ACM123", "tok/../.."); err == nil {
			t.Fatal("want an error")
		}
		if called {
			t.Fatal("a request was made with an unsafe segment")
		}
	})
}

func TestReportsInvoiceDetails(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "reports", "invoice_details"))

	t.Run("[happy] decodes the outstanding/paid/total summary", func(t *testing.T) {
		got, err := c.Reports.InvoiceDetails(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.DateType != "issue" || got.Summary.Total.Amount != "0.00" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[happy] docs-confirmed options encode as bare query params", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "invoice_details")(w, r)
		}))
		if _, err := c.Reports.InvoiceDetails(ctx, "ACM123", &InvoiceDetailsOptions{
			StartDate: "2019-01-01", EndDate: "2019-12-31", ClientIDs: "31006", StatusIDs: "paid", DateType: "issue",
		}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(gotQuery, "clientids=31006") || !strings.Contains(gotQuery, "date_type=issue") {
			t.Fatalf("query = %q", gotQuery)
		}
	})
}

func TestReportsItemSales(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] comma-separated filters encode as bare query params", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "item_sales")(w, r)
		}))
		got, err := c.Reports.ItemSales(ctx, "ACM123", &ItemSalesOptions{
			StartDate: "2019-01-01", EndDate: "2019-12-31", ClientIDs: "31006", StatusIDs: "paid",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(gotQuery, "clientids=31006") || !strings.Contains(gotQuery, "statusids=paid") {
			t.Fatalf("query = %q", gotQuery)
		}
		if len(got.Items) != 1 || got.Items[0].Invoices[0].Status != "overdue" {
			t.Fatalf("got = %+v", got.Items)
		}
		if got.Total == nil || got.Total.Amount != "23320.00" || got.TotalDiscount == nil || got.TotalDiscount.Amount != "0.00" {
			t.Fatalf("totals = %+v / %+v", got.Total, got.TotalDiscount)
		}
		if got.TotalQty != "20" {
			t.Fatalf("TotalQty = %q, want the captured decimal string", got.TotalQty)
		}
		if got.StartDate != "2019-01-01" || len(got.StatusIDs) != 0 {
			t.Fatalf("StartDate/StatusIDs = %q / %v", got.StartDate, got.StatusIDs)
		}
	})
}

func TestReportsPaymentsCollected(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "reports", "payments_collected"))

	t.Run("[edge] an empty period has no payments or totals", func(t *testing.T) {
		got, err := c.Reports.PaymentsCollected(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Payments) != 0 || len(got.Totals) != 0 {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[happy] docs-confirmed options encode as bare query params", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "payments_collected")(w, r)
		}))
		if _, err := c.Reports.PaymentsCollected(ctx, "ACM123", &PaymentsCollectedOptions{
			StartDate: "2019-04-24", EndDate: "2019-04-24", Locale: "en-US", PaymentMethods: "credit_card",
		}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(gotQuery, "locale=en-US") || !strings.Contains(gotQuery, "payment_methods=credit_card") {
			t.Fatalf("query = %q", gotQuery)
		}
	})
}

func TestReportsProfitLoss(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "reports", "profit_loss"))

	t.Run("[happy] the expense tree nests children", func(t *testing.T) {
		got, err := c.Reports.ProfitLoss(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Expenses) != 1 || len(got.Expenses[0].Children) != 1 {
			t.Fatalf("got = %+v", got.Expenses)
		}
		if got.Expenses[0].Children[0].Description != "Gas" {
			t.Fatalf("child = %+v", got.Expenses[0].Children[0])
		}
		if got.NetProfit.Total.Amount != "-141.12" {
			t.Fatalf("net profit = %+v", got.NetProfit)
		}
	})

	t.Run("[happy] date-range and currency options encode as bare query params", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "profit_loss")(w, r)
		}))
		if _, err := c.Reports.ProfitLoss(ctx, "ACM123", &ProfitLossOptions{
			StartDate: "2019-04-24", EndDate: "2019-04-24", CurrencyCode: "USD",
		}); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "currency_code=USD&end_date=2019-04-24&start_date=2019-04-24" {
			t.Fatalf("query = %q", gotQuery)
		}
	})
}

func TestReportsRevenueByClient(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] sends sales_type as a bare query param", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "revenue_by_client")(w, r)
		}))
		got, err := c.Reports.RevenueByClient(ctx, "ACM123", &RevenueByClientOptions{SalesType: "collected"})
		if err != nil {
			t.Fatal(err)
		}
		if gotQuery != "sales_type=collected" {
			t.Fatalf("query = %q", gotQuery)
		}
		if got.SalesType != "collected" || got.TotalSales.Amount != "0.00" {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[sad] a missing sales_type is rejected", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "reports", "revenue_by_client_error"))
		_, err := c.Reports.RevenueByClient(ctx, "ACM123", nil)
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestReportsSalesTaxSummary(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] cash_based encodes as a bare true/false string", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "reports", "sales_tax_summary")(w, r)
		}))
		cashBased := true
		got, err := c.Reports.SalesTaxSummary(ctx, "ACM123", &SalesTaxSummaryOptions{
			StartDate: "2019-01-01", EndDate: "2019-05-01", CurrencyCode: "USD", CashBased: &cashBased,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(gotQuery, "cash_based=true") {
			t.Fatalf("query = %q", gotQuery)
		}
		if len(got.Taxes) != 1 || got.Taxes[0].TaxName != "HST" {
			t.Fatalf("got = %+v", got.Taxes)
		}
	})
}

func TestReportsTrialBalance(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "reports", "trial_balance"))

	t.Run("[happy] decodes debit/credit lines", func(t *testing.T) {
		got, err := c.Reports.TrialBalance(ctx, "ACM123", &TrialBalanceOptions{StartDate: "2019-01-01", EndDate: "2019-12-31"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Data) != 2 || got.Data[0].Debit.Amount != "6350.55" {
			t.Fatalf("got = %+v", got.Data)
		}
		if got.DownloadToken == "" || got.StartDate != "2019-01-01" || got.EndDate != "2019-12-31" {
			t.Fatalf("DownloadToken/StartDate/EndDate = %q / %q / %q", got.DownloadToken, got.StartDate, got.EndDate)
		}
	})
}

func TestReportsTimeEntryDetails(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] business-scoped, flat with a meta pagination object", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "reports", "time_entry_details")(w, r)
		}))
		got, err := c.Reports.TimeEntryDetails(ctx, BusinessID(1))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/comments/business/1/time_entries/search" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(got.TimeEntries) != 1 || got.TimeEntries[0].Duration != 7200 {
			t.Fatalf("got = %+v", got.TimeEntries)
		}
		if !got.TimeEntries[0].IsLogged {
			t.Fatalf("is_logged did not decode: %+v", got.TimeEntries[0])
		}
		if got.Meta.Total != 9 || got.Meta.PerPage != 30 {
			t.Fatalf("meta = %+v", got.Meta)
		}
		if got.Meta.TotalLogged != 3 || got.Meta.TotalUnbilled != 2 {
			t.Fatalf("meta totals = %+v", got.Meta)
		}
		if got.DownloadToken == "" {
			t.Fatalf("DownloadToken did not decode: %+v", got)
		}
		if len(got.Abilities) != 1 || !got.Abilities[0].Abilities[0].Value {
			t.Fatalf("abilities = %+v", got.Abilities)
		}
	})
}

func TestReportsRejectUnsafeAccountID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		acct AccountID
	}{
		{"[sad] a path separator", "a/b"},
		{"[sad] a query delimiter", "a?b"},
		{"[sad] a fragment delimiter", "a#b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			if _, err := c.Reports.AccountsAging(ctx, tc.acct, nil); err == nil {
				t.Fatal("want an error")
			}
			if _, err := c.Reports.BalanceSheet(ctx, tc.acct, nil); err == nil {
				t.Fatal("want an error")
			}
			if _, err := c.Reports.TrialBalance(ctx, tc.acct, nil); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
