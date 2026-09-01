package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type reportsAccountsAgingIn struct {
	AcctScope
	Options freshbooks.AccountsAgingOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsBalanceSheetIn struct {
	AcctScope
	Options freshbooks.BalanceSheetOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsBankReconciliationSummaryIn struct {
	AcctScope
	Options freshbooks.BankReconciliationSummaryOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsClientAccountStatementIn struct {
	AcctScope
	Options freshbooks.ClientAccountStatementOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsDownloadInvoiceDetailsCSVIn struct {
	AcctScope
	DownloadToken string `json:"download_token" jsonschema:"the download token from an Invoice Details report response"`
}

type reportsExpenseDetailsIn struct {
	AcctScope
	Options freshbooks.ExpenseDetailsOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsInvoiceDetailsIn struct {
	AcctScope
	Options freshbooks.InvoiceDetailsOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsItemSalesIn struct {
	AcctScope
	Options freshbooks.ItemSalesOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsPaymentsCollectedIn struct {
	AcctScope
	Options freshbooks.PaymentsCollectedOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsProfitLossIn struct {
	AcctScope
	Options freshbooks.ProfitLossOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsRevenueByClientIn struct {
	AcctScope
	Options freshbooks.RevenueByClientOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsSalesTaxSummaryIn struct {
	AcctScope
	Options freshbooks.SalesTaxSummaryOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

type reportsTrialBalanceIn struct {
	AcctScope
	Options freshbooks.TrialBalanceOptions `json:"options,omitempty" jsonschema:"report filter options"`
}

// reportsSpecs are the tools wrapping *freshbooks.ReportsService.
var reportsSpecs = []Spec{
	newSpec("reports_accounts_aging",
		"Run the Accounts Aging report. See https://www.freshbooks.com/api/reports.",
		"Reports", "AccountsAging",
		[]string{"Reports/Accounts Aging"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsAccountsAgingIn) (any, error) {
			return c.Reports.AccountsAging(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_balance_sheet",
		"Run the Balance Sheet report. See https://www.freshbooks.com/api/reports.",
		"Reports", "BalanceSheet",
		[]string{"Reports/Balance Sheet"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsBalanceSheetIn) (any, error) {
			return c.Reports.BalanceSheet(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_bank_reconciliation_summary",
		"Run the Bank Reconciliation Summary report. See https://www.freshbooks.com/api/reports.",
		"Reports", "BankReconciliationSummary",
		[]string{"Reports/Bank Reconciliation Summary"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsBankReconciliationSummaryIn) (any, error) {
			return c.Reports.BankReconciliationSummary(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_client_account_statement",
		"Run the Client Account Statement report. See https://www.freshbooks.com/api/reports.",
		"Reports", "ClientAccountStatement",
		[]string{"Reports/Client Account Statement"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsClientAccountStatementIn) (any, error) {
			return c.Reports.ClientAccountStatement(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_download_invoice_details_csv",
		"Download an Invoice Details report as CSV, base64-encoded, using a download token from reports_invoice_details. See https://www.freshbooks.com/api/reports.",
		"Reports", "DownloadInvoiceDetailsCSV",
		[]string{"Reports/Download CSV Report"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsDownloadInvoiceDetailsCSVIn) (any, error) {
			data, err := c.Reports.DownloadInvoiceDetailsCSV(ctx, scope.AccountID, in.DownloadToken)
			if err != nil {
				return nil, err
			}
			return newBinaryResult("text/csv", data), nil
		}),
	newSpec("reports_expense_details",
		"Run the Expense Details report. See https://www.freshbooks.com/api/reports.",
		"Reports", "ExpenseDetails",
		[]string{"Reports/Expense Details"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsExpenseDetailsIn) (any, error) {
			return c.Reports.ExpenseDetails(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_invoice_details",
		"Run the Invoice Details report. See https://www.freshbooks.com/api/reports.",
		"Reports", "InvoiceDetails",
		[]string{"Reports/Invoice Details"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsInvoiceDetailsIn) (any, error) {
			return c.Reports.InvoiceDetails(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_item_sales",
		"Run the Item Sales report. See https://www.freshbooks.com/api/reports.",
		"Reports", "ItemSales",
		[]string{"Reports/Item Sales"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsItemSalesIn) (any, error) {
			return c.Reports.ItemSales(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_payments_collected",
		"Run the Payments Collected report. See https://www.freshbooks.com/api/reports.",
		"Reports", "PaymentsCollected",
		[]string{"Reports/Payments Collected"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsPaymentsCollectedIn) (any, error) {
			return c.Reports.PaymentsCollected(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_profit_loss",
		"Run the Profit and Loss report. See https://www.freshbooks.com/api/reports.",
		"Reports", "ProfitLoss",
		[]string{"Reports/Profit/Loss Report"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsProfitLossIn) (any, error) {
			return c.Reports.ProfitLoss(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_revenue_by_client",
		"Run the Revenue By Client report. See https://www.freshbooks.com/api/reports.",
		"Reports", "RevenueByClient",
		[]string{"Reports/Revenue By Client"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsRevenueByClientIn) (any, error) {
			return c.Reports.RevenueByClient(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_sales_tax_summary",
		"Run the Sales Tax Summary report. See https://www.freshbooks.com/api/reports.",
		"Reports", "SalesTaxSummary",
		[]string{"Reports/Sales Tax Summary"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsSalesTaxSummaryIn) (any, error) {
			return c.Reports.SalesTaxSummary(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_trial_balance",
		"Run the Trial Balance report. See https://www.freshbooks.com/api/reports.",
		"Reports", "TrialBalance",
		[]string{"Reports/Trial Balance"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in reportsTrialBalanceIn) (any, error) {
			return c.Reports.TrialBalance(ctx, scope.AccountID, &in.Options)
		}),
	newSpec("reports_time_entry_details",
		"Run the Time Entry Details report. See https://www.freshbooks.com/api/reports.",
		"Reports", "TimeEntryDetails",
		[]string{"Reports/Time Entry Details"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in BizScope) (any, error) {
			return c.Reports.TimeEntryDetails(ctx, scope.BusinessID)
		}),
}
