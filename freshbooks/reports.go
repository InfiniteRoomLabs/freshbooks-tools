package freshbooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// Every method's request struct exposes only the query parameters the
// Postman collection's examples actually exercised; a report with no
// evidenced parameters takes none -- use (*Client).Do for anything past
// what is modeled here.

// reportPath appends q to path as a query string, when q carries anything.
func reportPath(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}

// DatedBalance is one balance figure as of a specific date, the unit the
// balance-sheet report reports in.
type DatedBalance struct {
	Balance Money  `json:"balance"`
	Date    string `json:"date"`
}

// AgingBuckets buckets a balance by how overdue it is. Every field is a
// fixed FreshBooks bucket, not a caller-chosen range.
type AgingBuckets struct {
	Days0To30  Money `json:"0-30"`
	Days31To60 Money `json:"31-60"`
	Days61To90 Money `json:"61-90"`
	Days91Plus Money `json:"91+"`
	Total      Money `json:"total"`
}

// AccountsAgingClient is one client's outstanding balance, bucketed by age.
type AccountsAgingClient struct {
	AgingBuckets
	Email        string `json:"email"`
	FirstName    string `json:"fname"`
	LastName     string `json:"lname"`
	Organization string `json:"organization"`
	UserID       int64  `json:"userid"`
}

// AccountsAgingReport is the accounts-receivable aging report: each
// client's outstanding balance bucketed by how overdue it is, as of the
// most recent snapshot. The Postman example took no query parameters.
type AccountsAgingReport struct {
	Accounts      []AccountsAgingClient `json:"accounts"`
	CompanyName   string                `json:"company_name"`
	CurrencyCode  string                `json:"currency_code"`
	DownloadToken string                `json:"download_token"`
	EndDate       string                `json:"end_date"`
	Totals        AgingBuckets          `json:"totals"`
}

// AccountsAging returns the accounts-receivable aging report for acct.
//
// inventory: Reports/Accounts Aging
func (s *ReportsService) AccountsAging(ctx context.Context, acct AccountID) (*AccountsAgingReport, error) {
	path := "/accounting/account/" + string(acct) + "/reports/accounting/accounts_aging"
	var resp struct {
		AccountsAging AccountsAgingReport `json:"accounts_aging"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.AccountsAging, nil
}

// BalanceSheetOptions requests a balance sheet as of one or more dates, in
// one currency.
type BalanceSheetOptions struct {
	// Dates are the "as of" dates to report balances for, "YYYY-MM-DD".
	Dates []string
	// CurrencyCode restricts the report to one ISO 4217 currency.
	CurrencyCode string
}

func (o *BalanceSheetOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	for _, d := range o.Dates {
		q.Add("dates[]", d)
	}
	if o.CurrencyCode != "" {
		q.Set("currency_code", o.CurrencyCode)
	}
	return q
}

// BalanceSheetSubAccount is one sub-account's balances within a
// BalanceSheetAccount.
type BalanceSheetSubAccount struct {
	Balances         []DatedBalance `json:"balances"`
	SubAccountName   string         `json:"sub_account_name"`
	SubAccountNumber string         `json:"sub_account_number"`
}

// BalanceSheetAccount is one account's balances, with its sub-accounts,
// within a BalanceSheetGroup.
type BalanceSheetAccount struct {
	AccountName   string                   `json:"account_name"`
	AccountNumber string                   `json:"account_number"`
	Balances      []DatedBalance           `json:"balances"`
	SubAccounts   []BalanceSheetSubAccount `json:"sub_accounts"`
}

// BalanceSheetGroup is one top-level category (asset, liability, equity) of
// the balance sheet.
type BalanceSheetGroup struct {
	AccountType string                `json:"account_type"`
	Accounts    []BalanceSheetAccount `json:"accounts"`
	Balances    []DatedBalance        `json:"balances"`
	Category    string                `json:"category"`
}

// BalanceSheetReport is the balance-sheet report: assets, liabilities, and
// equity as of one or more dates.
type BalanceSheetReport struct {
	AssetsTotal               []DatedBalance      `json:"assets_total"`
	CompanyName               string              `json:"company_name"`
	CurrencyCode              string              `json:"currency_code"`
	Data                      []BalanceSheetGroup `json:"data"`
	Dates                     []string            `json:"dates"`
	DownloadToken             string              `json:"download_token"`
	LiabilitiesAndEquityTotal []DatedBalance      `json:"liabilities_and_equity_total"`
}

// BalanceSheet returns the balance sheet for acct.
//
// inventory: Reports/Balance Sheet
func (s *ReportsService) BalanceSheet(ctx context.Context, acct AccountID, opts *BalanceSheetOptions) (*BalanceSheetReport, error) {
	path := reportPath("/accounting/account/"+string(acct)+"/reports/accounting/balance_sheet", opts.values())
	var resp struct {
		BalanceSheet BalanceSheetReport `json:"balance_sheet"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.BalanceSheet, nil
}

// BankReconciliationSummaryOptions filters the bank-reconciliation-summary
// report.
type BankReconciliationSummaryOptions struct {
	CurrencyCode  string
	Date          string
	BankAccountID string
}

func (o *BankReconciliationSummaryOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	if o.CurrencyCode != "" {
		q.Set("currency_code", o.CurrencyCode)
	}
	if o.Date != "" {
		q.Set("date", o.Date)
	}
	if o.BankAccountID != "" {
		q.Set("bank_accountid", o.BankAccountID)
	}
	return q
}

// BankReconciliationSummary returns the bank-reconciliation-summary report
// for acct. The Postman collection carries no example response for this
// report and it has no public FreshBooks docs page, so its body shape is
// unknown; raw holds the report object exactly as the API returned it for
// the caller to unmarshal further.
//
// inventory: Reports/Bank Reconciliation Summary
func (s *ReportsService) BankReconciliationSummary(ctx context.Context, acct AccountID, opts *BankReconciliationSummaryOptions) (raw json.RawMessage, err error) {
	path := reportPath("/accounting/account/"+string(acct)+"/reports/accounting/bank_reconciliation_summary", opts.values())
	var resp struct {
		BankReconciliationSummary json.RawMessage `json:"bank_reconciliation_summary"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return resp.BankReconciliationSummary, nil
}

// ClientAccountStatementOptions filters the client-account-statement
// report.
type ClientAccountStatementOptions struct {
	StartDate    string
	EndDate      string
	ClientID     string
	CurrencyCode string
}

func (o *ClientAccountStatementOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	if o.StartDate != "" {
		q.Set("start_date", o.StartDate)
	}
	if o.EndDate != "" {
		q.Set("end_date", o.EndDate)
	}
	if o.ClientID != "" {
		q.Set("clientid", o.ClientID)
	}
	if o.CurrencyCode != "" {
		q.Set("currency_code", o.CurrencyCode)
	}
	return q
}

// ClientAccountStatement returns the client-account-statement report for
// acct. Like BankReconciliationSummary, the Postman collection carries no
// example response, so raw holds the report object unparsed.
//
// inventory: Reports/Client Account Statement
func (s *ReportsService) ClientAccountStatement(ctx context.Context, acct AccountID, opts *ClientAccountStatementOptions) (raw json.RawMessage, err error) {
	path := reportPath("/accounting/account/"+string(acct)+"/reports/accounting/account_statement", opts.values())
	var resp struct {
		AccountStatement json.RawMessage `json:"account_statement"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return resp.AccountStatement, nil
}

// DownloadCSV fetches a report's CSV export by the download token embedded
// in that report's DownloadToken field. The response is a CSV file, not
// JSON, so it bypasses envelope unwrapping entirely; the caller decides
// what to do with the bytes (write to disk, parse, etc).
//
// inventory: Reports/Download CSV Report
func (s *ReportsService) DownloadCSV(ctx context.Context, acct AccountID, downloadToken string) ([]byte, error) {
	path := "/accounting/account/" + string(acct) + "/links/reports/" + downloadToken + "/invoice_details.csv"
	return s.client.doRaw(ctx, http.MethodGet, path, FamilyAccounting)
}

// ExpenseDetails returns the expense-details report for acct. The Postman
// collection carries no example response for this exact endpoint, and the
// FreshBooks expense-details-report docs page describes a differently
// shaped, business_uuid-scoped endpoint that does not match this
// AccountID-scoped one -- the two disagree, and this method follows the
// Postman inventory (the parity contract) rather than the docs page. raw
// holds the report object unparsed until that discrepancy is resolved live.
//
// inventory: Reports/Expense Details
func (s *ReportsService) ExpenseDetails(ctx context.Context, acct AccountID) (raw json.RawMessage, err error) {
	path := "/accounting/account/" + string(acct) + "/reports/accounting/expense_details"
	var resp struct {
		ExpenseDetails json.RawMessage `json:"expense_details"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return resp.ExpenseDetails, nil
}

// ReportSummary is the outstanding/paid/total breakdown InvoiceDetails
// reports in its summary.
type ReportSummary struct {
	Outstanding Money `json:"outstanding"`
	Paid        Money `json:"paid"`
	Total       Money `json:"total"`
}

// InvoiceDetailsReport is the invoice-details report: every invoice in the
// period plus an outstanding/paid/total summary.
type InvoiceDetailsReport struct {
	ClientIDs     []int64       `json:"clientids"`
	CompanyName   string        `json:"company_name"`
	CurrencyCode  string        `json:"currency_code"`
	DateType      string        `json:"date_type"`
	DownloadToken string        `json:"download_token"`
	EndDate       string        `json:"end_date"`
	StartDate     string        `json:"start_date"`
	StatusIDs     []string      `json:"statusids"`
	Summary       ReportSummary `json:"summary"`
	SummaryOnly   bool          `json:"summary_only"`
}

// InvoiceDetails returns the invoice-details report for acct.
//
// inventory: Reports/Invoice Details
func (s *ReportsService) InvoiceDetails(ctx context.Context, acct AccountID) (*InvoiceDetailsReport, error) {
	path := "/accounting/account/" + string(acct) + "/reports/accounting/invoice_details"
	var resp struct {
		InvoiceDetails InvoiceDetailsReport `json:"invoice_details"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.InvoiceDetails, nil
}

// ItemSalesOptions filters the item-sales report. ClientIDs, ItemNames, and
// StatusIDs are comma-separated, matching the Postman example's single
// (not array) query values.
type ItemSalesOptions struct {
	StartDate    string
	EndDate      string
	CurrencyCode string
	ClientIDs    string
	ItemNames    string
	StatusIDs    string
}

func (o *ItemSalesOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	for k, v := range map[string]string{
		"start_date": o.StartDate, "end_date": o.EndDate, "currency_code": o.CurrencyCode,
		"clientids": o.ClientIDs, "item_names": o.ItemNames, "statusids": o.StatusIDs,
	} {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q
}

// ItemSaleInvoice is one invoice line an item was sold on, within an
// ItemSale.
type ItemSaleInvoice struct {
	Amount        Money  `json:"amount"`
	CreateDate    string `json:"create_date"`
	Discount      Money  `json:"discount"`
	FirstName     string `json:"fname"`
	InvoiceNumber string `json:"invoice_number"`
	InvoiceID     int64  `json:"invoiceid"`
	LastName      string `json:"lname"`
	Organization  string `json:"organization"`
	Qty           string `json:"qty"`
	UnitCost      Money  `json:"unit_cost"`
	Status        string `json:"v3_status"`
}

// ItemSale is one item's total sales within the report period.
type ItemSale struct {
	Invoices      []ItemSaleInvoice `json:"invoices"`
	Name          string            `json:"name"`
	Total         Money             `json:"total"`
	TotalDiscount Money             `json:"total_discount"`
	TotalQty      string            `json:"total_qty"`
}

// ItemSalesReport is the item-sales report: sales totals per item.
type ItemSalesReport struct {
	ClientIDs     []int64    `json:"clientids"`
	CompanyName   string     `json:"company_name"`
	CurrencyCode  string     `json:"currency_code"`
	DownloadToken string     `json:"download_token"`
	EndDate       string     `json:"end_date"`
	ItemNames     []string   `json:"item_names"`
	Items         []ItemSale `json:"items"`
}

// ItemSales returns the item-sales report for acct.
//
// inventory: Reports/Item Sales
func (s *ReportsService) ItemSales(ctx context.Context, acct AccountID, opts *ItemSalesOptions) (*ItemSalesReport, error) {
	path := reportPath("/accounting/account/"+string(acct)+"/reports/accounting/item_sales", opts.values())
	var resp struct {
		ItemSales ItemSalesReport `json:"item_sales"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.ItemSales, nil
}

// PaymentsCollectedReport is the payments-collected report. Payments and
// Totals are left as raw JSON: the Postman example returned them empty, so
// their populated element shape is unknown.
type PaymentsCollectedReport struct {
	ClientIDs      []int64           `json:"clientids"`
	CurrencyCodes  []string          `json:"currency_codes"`
	DownloadToken  string            `json:"download_token"`
	EndDate        string            `json:"end_date"`
	PaymentMethods []string          `json:"payment_methods"`
	Payments       []json.RawMessage `json:"payments"`
	StartDate      string            `json:"start_date"`
	Totals         []json.RawMessage `json:"totals"`
}

// PaymentsCollected returns the payments-collected report for acct.
//
// inventory: Reports/Payments Collected
func (s *ReportsService) PaymentsCollected(ctx context.Context, acct AccountID) (*PaymentsCollectedReport, error) {
	path := "/accounting/account/" + string(acct) + "/reports/accounting/payments_collected"
	var resp struct {
		PaymentsCollected PaymentsCollectedReport `json:"payments_collected"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.PaymentsCollected, nil
}

// ProfitLossLine is one row of the profit-and-loss report: a labeled total,
// optionally broken down into child rows.
type ProfitLossLine struct {
	Children    []ProfitLossLine  `json:"children"`
	Data        []json.RawMessage `json:"data"`
	Description string            `json:"description"`
	EntryType   string            `json:"entry_type"`
	Total       Money             `json:"total"`
}

// ProfitLossReport is the profit-and-loss report.
type ProfitLossReport struct {
	CashBased     bool             `json:"cash_based"`
	CompanyName   string           `json:"company_name"`
	CurrencyCode  string           `json:"currency_code"`
	DownloadToken string           `json:"download_token"`
	EndDate       string           `json:"end_date"`
	Expenses      []ProfitLossLine `json:"expenses"`
	Income        []ProfitLossLine `json:"income"`
	Labels        []string         `json:"labels"`
	NetProfit     ProfitLossLine   `json:"net_profit"`
	Resolution    *string          `json:"resolution"`
	StartDate     string           `json:"start_date"`
	TotalExpenses ProfitLossLine   `json:"total_expenses"`
	TotalIncome   ProfitLossLine   `json:"total_income"`
}

// ProfitLoss returns the profit-and-loss report for acct.
//
// inventory: Reports/Profit/Loss Report
func (s *ReportsService) ProfitLoss(ctx context.Context, acct AccountID) (*ProfitLossReport, error) {
	path := "/accounting/account/" + string(acct) + "/reports/accounting/profitloss_entity"
	var resp struct {
		ProfitLoss ProfitLossReport `json:"profitloss"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.ProfitLoss, nil
}

// RevenueByClientOptions filters the revenue-by-client report.
type RevenueByClientOptions struct {
	// SalesType selects how revenue is recognized, e.g. "collected".
	SalesType string
}

func (o *RevenueByClientOptions) values() url.Values {
	q := url.Values{}
	if o != nil && o.SalesType != "" {
		q.Set("sales_type", o.SalesType)
	}
	return q
}

// RevenueByClientReport is the revenue-by-client report.
type RevenueByClientReport struct {
	ClientIDs      []int64           `json:"clientids"`
	Clients        []json.RawMessage `json:"clients"`
	CurrencyCode   string            `json:"currency_code"`
	DownloadToken  string            `json:"download_token"`
	EndDate        string            `json:"end_date"`
	Labels         []string          `json:"labels"`
	SalesType      string            `json:"sales_type"`
	StartDate      string            `json:"start_date"`
	TotalSales     Money             `json:"total_sales"`
	TotalSalesData []Money           `json:"total_sales_data"`
}

// RevenueByClient returns the revenue-by-client report for acct. SalesType
// is required -- an empty or missing value is rejected by the API.
//
// inventory: Reports/Revenue By Client
func (s *ReportsService) RevenueByClient(ctx context.Context, acct AccountID, opts *RevenueByClientOptions) (*RevenueByClientReport, error) {
	path := reportPath("/accounting/account/"+string(acct)+"/reports/accounting/revenue_by_client", opts.values())
	var resp struct {
		RevenueByClient RevenueByClientReport `json:"revenue_by_client"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.RevenueByClient, nil
}

// SalesTaxSummaryOptions filters the sales-tax-summary report.
type SalesTaxSummaryOptions struct {
	StartDate    string
	EndDate      string
	CurrencyCode string
	// CashBased selects cash-basis (true) or accrual-basis (false)
	// accounting; nil omits the parameter.
	CashBased *bool
}

func (o *SalesTaxSummaryOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	if o.StartDate != "" {
		q.Set("start_date", o.StartDate)
	}
	if o.EndDate != "" {
		q.Set("end_date", o.EndDate)
	}
	if o.CurrencyCode != "" {
		q.Set("currency_code", o.CurrencyCode)
	}
	if o.CashBased != nil {
		q.Set("cash_based", boolQuery(*o.CashBased))
	}
	return q
}

func boolQuery(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// SalesTaxSummaryLine is one tax's collected/paid/net breakdown within a
// SalesTaxSummaryReport.
type SalesTaxSummaryLine struct {
	NetTax                 Money  `json:"net_tax"`
	NetTaxableAmount       Money  `json:"net_taxable_amount"`
	TaxCollected           Money  `json:"tax_collected"`
	TaxName                string `json:"tax_name"`
	TaxPaid                Money  `json:"tax_paid"`
	TaxableAmountCollected Money  `json:"taxable_amount_collected"`
	TaxableAmountPaid      Money  `json:"taxable_amount_paid"`
}

// SalesTaxSummaryReport is the sales-tax-summary report.
type SalesTaxSummaryReport struct {
	CashBased     bool                  `json:"cash_based"`
	CurrencyCode  string                `json:"currency_code"`
	DownloadToken string                `json:"download_token"`
	EndDate       string                `json:"end_date"`
	StartDate     string                `json:"start_date"`
	Taxes         []SalesTaxSummaryLine `json:"taxes"`
	TotalInvoiced Money                 `json:"total_invoiced"`
}

// SalesTaxSummary returns the sales-tax-summary report for acct.
//
// inventory: Reports/Sales Tax Summary
func (s *ReportsService) SalesTaxSummary(ctx context.Context, acct AccountID, opts *SalesTaxSummaryOptions) (*SalesTaxSummaryReport, error) {
	path := reportPath("/accounting/account/"+string(acct)+"/reports/accounting/taxsummary", opts.values())
	var resp struct {
		TaxSummary SalesTaxSummaryReport `json:"taxsummary"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.TaxSummary, nil
}

// TrialBalanceOptions filters the trial-balance report.
type TrialBalanceOptions struct {
	StartDate    string
	EndDate      string
	CurrencyCode string
}

func (o *TrialBalanceOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	if o.StartDate != "" {
		q.Set("start_date", o.StartDate)
	}
	if o.EndDate != "" {
		q.Set("end_date", o.EndDate)
	}
	if o.CurrencyCode != "" {
		q.Set("currency_code", o.CurrencyCode)
	}
	return q
}

// TrialBalanceLine is one sub-account's debit/credit balance within a
// TrialBalanceReport.
type TrialBalanceLine struct {
	AccountName      string `json:"account_name"`
	AccountNumber    string `json:"account_number"`
	AccountSubName   string `json:"account_sub_name"`
	AccountSubNumber string `json:"account_sub_number"`
	Credit           Money  `json:"credit"`
	Debit            Money  `json:"debit"`
	SubAccountID     int64  `json:"sub_accountid"`
}

// TrialBalanceReport is the trial-balance report.
type TrialBalanceReport struct {
	CompanyName  string             `json:"company_name"`
	CurrencyCode string             `json:"currency_code"`
	Data         []TrialBalanceLine `json:"data"`
}

// TrialBalance returns the trial-balance report for acct.
//
// inventory: Reports/Trial Balance
func (s *ReportsService) TrialBalance(ctx context.Context, acct AccountID, opts *TrialBalanceOptions) (*TrialBalanceReport, error) {
	path := reportPath("/accounting/account/"+string(acct)+"/reports/accounting/trial_balance", opts.values())
	var resp struct {
		TrialBalance TrialBalanceReport `json:"trial_balance"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.TrialBalance, nil
}

// TimeEntryAbility is one action ("can_edit", "can_delete", ...) and
// whether the caller is allowed to take it.
type TimeEntryAbility struct {
	Name  string `json:"name"`
	Value bool   `json:"value"`
}

// TimeEntryAbilities lists what the caller may do with one time entry.
type TimeEntryAbilities struct {
	TimeEntryID int64              `json:"time_entry_id"`
	Abilities   []TimeEntryAbility `json:"abilities"`
}

// TimeEntryDetailEntry is one logged time entry within a
// TimeEntryDetailsReport.
type TimeEntryDetailEntry struct {
	ID         int64     `json:"id"`
	IdentityID int64     `json:"identity_id"`
	StartedAt  time.Time `json:"started_at"`
	CreatedAt  time.Time `json:"created_at"`
	Duration   int64     `json:"duration"`
	ClientID   int64     `json:"client_id"`
	ProjectID  int64     `json:"project_id"`
	TaskID     *int64    `json:"task_id"`
	ServiceID  *int64    `json:"service_id"`
	Note       string    `json:"note"`
	Active     bool      `json:"active"`
	Billable   bool      `json:"billable"`
	Billed     bool      `json:"billed"`
	Internal   bool      `json:"internal"`
	RetainerID *int64    `json:"retainer_id"`
}

// TimeEntryDetailsReport is the time-entry-details report: logged time
// entries plus what the caller may do with each. Unlike every other report
// in this file it is business-scoped (BusinessID, not AccountID) and its
// response is flat with a "meta" pagination object, not the accounting
// envelope.
type TimeEntryDetailsReport struct {
	TimeEntries []TimeEntryDetailEntry `json:"time_entries"`
	Meta        PageMeta               `json:"meta"`
	Abilities   []TimeEntryAbilities   `json:"abilities"`
}

// TimeEntryDetails returns the time-entry-details report for biz.
//
// inventory: Reports/Time Entry Details
func (s *ReportsService) TimeEntryDetails(ctx context.Context, biz BusinessID) (*TimeEntryDetailsReport, error) {
	path := "/comments/business/" + biz.String() + "/time_entries/search"
	var resp TimeEntryDetailsReport
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
