package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// reportsCommands wrap *freshbooks.ReportsService.
//
// docs/phases/4/commands.md's "report opts as flags" column is not
// implemented as individual per-report flags: each report's filter
// struct has its own bespoke field set (date ranges, currency, grouping),
// and 13 distinct field-to-flag mappings was out of scope for this pass.
// Instead every report command accepts its filter options the same way a
// write command accepts a body -- an optional -f/--file JSON document
// decoded into the report's *XxxOptions struct -- via
// Invocation.DecodeBodyOptional, so the full filtering capability is
// still reachable, just through one flag instead of many. See the
// implementer report.
var reportsCommands = []Command{
	{
		Group: "reports", Verb: "accounts-aging",
		Short:   "Get the accounts aging report",
		Service: "Reports", Method: "AccountsAging",
		Keys:  []string{"Reports/Accounts Aging"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.AccountsAgingOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.AccountsAging(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.AccountsAging(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "balance-sheet",
		Short:   "Get the balance sheet report",
		Service: "Reports", Method: "BalanceSheet",
		Keys:  []string{"Reports/Balance Sheet"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.BalanceSheetOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.BalanceSheet(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.BalanceSheet(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "bank-reconciliation-summary",
		Short:   "Get the bank reconciliation summary report",
		Service: "Reports", Method: "BankReconciliationSummary",
		Keys:  []string{"Reports/Bank Reconciliation Summary"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.BankReconciliationSummaryOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.BankReconciliationSummary(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.BankReconciliationSummary(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "client-account-statement",
		Short:   "Get a client's account statement",
		Service: "Reports", Method: "ClientAccountStatement",
		Keys:  []string{"Reports/Client Account Statement"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.ClientAccountStatementOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.ClientAccountStatement(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.ClientAccountStatement(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "download-invoice-details-csv",
		Short:   "Download a previously requested invoice-details CSV export",
		Service: "Reports", Method: "DownloadInvoiceDetailsCSV",
		Keys:  []string{"Reports/Download CSV Report"},
		Class: ClassRO, Scope: ScopeAccount, ExtraPositional: []string{"download-token"}, Binary: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Reports.DownloadInvoiceDetailsCSV(ctx, inv.Scope.AccountID, inv.Extra(0))
		},
	},
	{
		Group: "reports", Verb: "expense-details",
		Short:   "Get the expense details report",
		Service: "Reports", Method: "ExpenseDetails",
		Keys:  []string{"Reports/Expense Details"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.ExpenseDetailsOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.ExpenseDetails(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.ExpenseDetails(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "invoice-details",
		Short:   "Get the invoice details report",
		Service: "Reports", Method: "InvoiceDetails",
		Keys:  []string{"Reports/Invoice Details"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.InvoiceDetailsOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.InvoiceDetails(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.InvoiceDetails(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "item-sales",
		Short:   "Get the item sales report",
		Service: "Reports", Method: "ItemSales",
		Keys:  []string{"Reports/Item Sales"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.ItemSalesOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.ItemSales(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.ItemSales(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "payments-collected",
		Short:   "Get the payments collected report",
		Service: "Reports", Method: "PaymentsCollected",
		Keys:  []string{"Reports/Payments Collected"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.PaymentsCollectedOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.PaymentsCollected(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.PaymentsCollected(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "profit-loss",
		Short:   "Get the profit/loss report",
		Service: "Reports", Method: "ProfitLoss",
		Keys:  []string{"Reports/Profit/Loss Report"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.ProfitLossOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.ProfitLoss(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.ProfitLoss(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "revenue-by-client",
		Short:   "Get the revenue by client report",
		Service: "Reports", Method: "RevenueByClient",
		Keys:  []string{"Reports/Revenue By Client"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.RevenueByClientOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.RevenueByClient(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.RevenueByClient(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "sales-tax-summary",
		Short:   "Get the sales tax summary report",
		Service: "Reports", Method: "SalesTaxSummary",
		Keys:  []string{"Reports/Sales Tax Summary"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.SalesTaxSummaryOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.SalesTaxSummary(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.SalesTaxSummary(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "trial-balance",
		Short:   "Get the trial balance report",
		Service: "Reports", Method: "TrialBalance",
		Keys:  []string{"Reports/Trial Balance"},
		Class: ClassRO, Scope: ScopeAccount, Body: true, BodyOptional: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var opts freshbooks.TrialBalanceOptions
			has, err := inv.DecodeBodyOptional(&opts)
			if err != nil {
				return nil, err
			}
			if !has {
				return c.Reports.TrialBalance(ctx, inv.Scope.AccountID, nil)
			}
			return c.Reports.TrialBalance(ctx, inv.Scope.AccountID, &opts)
		},
	},
	{
		Group: "reports", Verb: "time-entry-details",
		Short:   "Get the time entry details report",
		Service: "Reports", Method: "TimeEntryDetails",
		Keys:  []string{"Reports/Time Entry Details"},
		Class: ClassRO, Scope: ScopeBusiness,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Reports.TimeEntryDetails(ctx, inv.Scope.BusinessID)
		},
	},
}
