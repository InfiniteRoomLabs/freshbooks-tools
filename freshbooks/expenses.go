package freshbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// ExpensesService is the expenses resource.
type ExpensesService struct{ client *Client }

// ExpenseAttachment is a receipt image already uploaded via the images
// endpoint (batch d), referenced by its JWT. AttachmentID and ID are numeric
// on read (every captured expense response sends them as bare JSON
// numbers); ExpenseAttachmentRequest keeps its own JWT-only write shape.
type ExpenseAttachment struct {
	AttachmentID int64  `json:"attachmentid,omitempty"`
	ID           int64  `json:"id,omitempty"`
	JWT          string `json:"jwt,omitempty"`
	MediaType    string `json:"media_type,omitempty"`
}

// Expense is a recorded business expense.
type Expense struct {
	// ID and ExpenseID are the same value under two names; the API returns
	// both.
	ID        int64 `json:"id"`
	ExpenseID int64 `json:"expenseid"`
	// Amount is the expense total.
	Amount Money `json:"amount"`
	// Date is when the expense was incurred.
	Date Date `json:"date"`
	// CategoryID files the expense under a category; Category is the same
	// category as a full nested object, present alongside CategoryID on
	// every captured response.
	CategoryID int64            `json:"categoryid,omitempty"`
	Category   *ExpenseCategory `json:"category,omitempty"`
	// ClientID bills this expense to a client, when set.
	ClientID int64 `json:"clientid,omitempty"`
	// ProjectID associates the expense with a project, when set.
	ProjectID int64 `json:"projectid,omitempty"`
	// StaffID is the staff member who incurred the expense.
	StaffID int64 `json:"staffid,omitempty"`
	// InvoiceID is set once the expense has been invoiced to a client.
	InvoiceID int64 `json:"invoiceid,omitempty"`
	// Vendor is the free-text vendor name.
	Vendor string `json:"vendor,omitempty"`
	// Notes is a free-text description.
	Notes string `json:"notes,omitempty"`
	// Status is 0 internal, 1 outstanding, 2 invoiced, 4 recouped.
	Status int `json:"status,omitempty"`
	// HasReceipt and IncludeReceipt report whether a receipt is attached
	// and whether it should be included when this expense is invoiced.
	HasReceipt     bool `json:"has_receipt,omitempty"`
	IncludeReceipt bool `json:"include_receipt,omitempty"`
	// IsCOGS marks the expense as cost of goods sold.
	IsCOGS bool `json:"is_cogs,omitempty"`
	// CompoundedTax reports whether TaxName2 compounds on top of TaxName1.
	CompoundedTax bool `json:"compounded_tax,omitempty"`
	// TaxName1, TaxName2, TaxAmount1, TaxAmount2, TaxPercent1, and
	// TaxPercent2 record up to two taxes applied to the expense.
	TaxName1    string `json:"taxName1,omitempty"`
	TaxName2    string `json:"taxName2,omitempty"`
	TaxAmount1  Money  `json:"taxAmount1,omitzero"`
	TaxAmount2  Money  `json:"taxAmount2,omitzero"`
	TaxPercent1 string `json:"taxPercent1,omitempty"`
	TaxPercent2 string `json:"taxPercent2,omitempty"`
	// MarkupPercent is the markup applied when this expense is billed
	// through to a client.
	MarkupPercent string `json:"markup_percent,omitempty"`
	// Attachment is the receipt image, when one was uploaded.
	Attachment *ExpenseAttachment `json:"attachment,omitempty"`
	// TransactionID links this expense to a bank transaction, when FreshBooks
	// matched one.
	TransactionID *int64 `json:"transactionid,omitempty"`
	// IsDuplicate reports whether FreshBooks flagged this expense as a
	// likely duplicate of another.
	IsDuplicate *bool `json:"isduplicate,omitempty"`
	// FromBulkImport reports whether this expense came from a bulk import
	// rather than being entered individually.
	FromBulkImport *bool `json:"from_bulk_import,omitempty"`
	// ProfileID links this expense back to the ExpenseProfile that
	// generated it, when it was created by a recurring-expense schedule.
	ProfileID *int64 `json:"profileid,omitempty"`
	// Updated is the account-local last-modified timestamp.
	Updated DateTime `json:"updated,omitempty"`
	// VisState is the expense's visibility state.
	VisState VisState `json:"vis_state"`

	// The fourteen fields below were on the wire but carried no struct tag
	// until Phase 8 convergence (2026-09-03, docs/progress.md backlog item
	// 14, live capture freshbooks/testdata/seed/expenses/list.json).
	// Following this struct's existing convention: a pointer where the
	// capture shows null (no evidence of the non-null shape -- INFERRED),
	// a value type with omitempty where the capture shows a present
	// zero/empty value.

	// AccountingSystemID mirrors the AccountID the expense was fetched
	// under (e.g. the account slug); present on every captured expense.
	AccountingSystemID string `json:"accounting_systemid,omitempty"`
	// AccountName is free text, empty on every captured expense.
	AccountName string `json:"account_name,omitempty"`
	// LegacyAccountID is a second, distinct account identifier from
	// AccountingSystemID -- always null on the capture, so its non-null
	// shape is unconfirmed. INFERRED as *string.
	LegacyAccountID *string `json:"accountid,omitempty"`
	// BackgroundJobID links the expense to an async job, when one is
	// running against it. Always null on the capture; typed *int64 to
	// match this struct's other *_id fields (TransactionID, ProfileID).
	// INFERRED.
	BackgroundJobID *int64 `json:"background_jobid,omitempty"`
	// BankName is free text, empty on every captured expense.
	BankName string `json:"bank_name,omitempty"`
	// BillMatches lists bank-transaction matches FreshBooks proposed for
	// this expense; always empty on the capture, so the element shape is
	// unconfirmed. INFERRED as one json.RawMessage per entry.
	BillMatches []json.RawMessage `json:"bill_matches,omitempty"`
	// Billable reports whether this expense can be billed through to a
	// client -- of the fourteen, the one a caller was most likely to have
	// missed before Phase 8.
	Billable bool `json:"billable,omitempty"`
	// ConverseProjectID and ModernProjectID are two more project-linkage
	// ids beside ProjectID, both always null on the capture. Typed *int64
	// to match ProjectID. INFERRED.
	ConverseProjectID *int64 `json:"converse_projectid,omitempty"`
	ModernProjectID   *int64 `json:"modern_projectid,omitempty"`
	// ExtAccountID is an external-system account identifier, always null
	// on the capture. INFERRED as *string.
	ExtAccountID *string `json:"ext_accountid,omitempty"`
	// ExtInvoiceID and ExtSystemID are external-system linkage ids, both 0
	// (present, not null) on every captured expense.
	ExtInvoiceID int64 `json:"ext_invoiceid,omitempty"`
	ExtSystemID  int64 `json:"ext_systemid,omitempty"`
	// PotentialBillPayment reports whether FreshBooks flagged this expense
	// as a possible bill payment rather than a plain expense.
	PotentialBillPayment bool `json:"potential_bill_payment,omitempty"`
	// Version is an account-local revision stamp, distinct from Updated.
	// Its wire format is a space-separated timestamp with fractional
	// seconds ("2026-08-28 00:00:00.000000") that DateTime does not model
	// (spec 5.1's STATE AS OF 2026-09-03 (Phase 7, live) callout already
	// recorded this for the sibling Invoice.Version field), so this stays
	// a plain string.
	Version string `json:"version,omitempty"`
}

type expenseEnvelope struct {
	Expense Expense `json:"expense"`
}

type expenseListEnvelope struct {
	Expenses []Expense `json:"expenses"`
	PageMeta
}

// ExpenseAttachmentRequest references an already-uploaded receipt image by
// its JWT (see the batch d Images service).
type ExpenseAttachmentRequest struct {
	JWT       string `json:"jwt"`
	MediaType string `json:"media_type,omitempty"`
}

// ExpenseWriteRequest is the payload shared by Create and Update. Every
// field is optional so a caller can send only what it means to set; Amount
// and Date are effectively required by the API for Create.
//
// CategoryID is *int64: the FreshBooks docs field table types it int and
// writable, and the docs page's own create-expense example sends it
// unquoted. An earlier version of this type sent it as a string, following
// a Postman example that happens to quote the value; the docs win where the
// two disagree (CLAUDE.md's "docs beat Postman" rule). TaxPercent1,
// TaxPercent2, and MarkupPercent are *string for the same reason: the docs
// field table types all three as string, matching the Expense read model.
type ExpenseWriteRequest struct {
	Amount        *Money                    `json:"amount,omitempty"`
	Date          *Date                     `json:"date,omitempty"`
	CategoryID    *int64                    `json:"categoryid,omitempty"`
	ClientID      *int64                    `json:"clientid,omitempty"`
	ProjectID     *int64                    `json:"projectid,omitempty"`
	StaffID       *int64                    `json:"staffid,omitempty"`
	Vendor        string                    `json:"vendor,omitempty"`
	Notes         string                    `json:"notes,omitempty"`
	TaxName1      string                    `json:"taxName1,omitempty"`
	TaxName2      string                    `json:"taxName2,omitempty"`
	TaxAmount1    *Money                    `json:"taxAmount1,omitempty"`
	TaxAmount2    *Money                    `json:"taxAmount2,omitempty"`
	TaxPercent1   *string                   `json:"taxPercent1,omitempty"`
	TaxPercent2   *string                   `json:"taxPercent2,omitempty"`
	MarkupPercent *string                   `json:"markup_percent,omitempty"`
	IsCOGS        *bool                     `json:"is_cogs,omitempty"`
	Attachment    *ExpenseAttachmentRequest `json:"attachment,omitempty"`
}

// ExpenseListOptions filters and paginates List.
type ExpenseListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *ExpenseListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

func expensesPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/expenses/expenses", acct), nil
}

func expensePath(acct AccountID, id int64) (string, error) {
	base, err := expensesPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}

// List returns one page of expenses.
//
// inventory: Expenses/List Expenses
func (s *ExpensesService) List(ctx context.Context, acct AccountID, opts *ExpenseListOptions, extra ...RequestOption) (*Page[Expense], error) {
	path, err := expensesPath(acct)
	if err != nil {
		return nil, err
	}
	var env expenseListEnvelope
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(env.Expenses, env.PageMeta), nil
}

// All walks every page of expenses, auto-paginating.
func (s *ExpensesService) All(ctx context.Context, acct AccountID, opts *ExpenseListOptions, extra ...RequestOption) iter.Seq2[Expense, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Expense], error) {
		o := ExpenseListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		o.PerPage = pageSize(o.PerPage)
		return s.List(ctx, acct, &o, extra...)
	})
}

// Get retrieves a single expense.
//
// inventory: Expenses/Single Expense
func (s *ExpensesService) Get(ctx context.Context, acct AccountID, id int64, opts ...RequestOption) (*Expense, error) {
	path, err := expensePath(acct, id)
	if err != nil {
		return nil, err
	}
	var env expenseEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, opts...); err != nil {
		return nil, err
	}
	return &env.Expense, nil
}

// Create records a new expense. The Postman collection carries this request
// twice -- once plain, once as "with Receipt" -- which differ only in
// whether Attachment is set on the same shared request type, so both keys
// stack on this one method.
//
// inventory: Expenses/Create Expense
// inventory: Expenses/Create Expense with Receipt
func (s *ExpensesService) Create(ctx context.Context, acct AccountID, req *ExpenseWriteRequest) (*Expense, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Expenses.Create needs a request")
	}
	path, err := expensesPath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		Expense *ExpenseWriteRequest `json:"expense"`
	}{req}
	var env expenseEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Expense, nil
}

// Update changes an existing expense. Like Create, "Update Expense" and
// "Update Expense with Receipt" share this one method.
//
// inventory: Expenses/Update Expense
// inventory: Expenses/Update Expense with Receipt
func (s *ExpensesService) Update(ctx context.Context, acct AccountID, id int64, req *ExpenseWriteRequest) (*Expense, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Expenses.Update needs a request")
	}
	path, err := expensePath(acct, id)
	if err != nil {
		return nil, err
	}
	body := struct {
		Expense *ExpenseWriteRequest `json:"expense"`
	}{req}
	var env expenseEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Expense, nil
}

// Delete soft-deletes an expense by setting vis_state to 1.
//
// The Postman collection's "Delete Expense" example body actually sends
// vis_state 0 (active), which contradicts every other soft-delete in this
// API family (bills, vendors, credit notes, estimates all delete via
// vis_state 1) and contradicts the FreshBooks docs page for this resource,
// which also says vis_state 1 means deleted. Treated as a Postman authoring
// mistake: this method sends vis_state 1. INFERRED; flagged in the Phase 2
// batch b report for a live check.
//
// inventory: Expenses/Delete Expense
func (s *ExpensesService) Delete(ctx context.Context, acct AccountID, id int64) error {
	path, err := expensePath(acct, id)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "expense")
}

// ExpenseSummaryAmount is one currency's contribution to an ExpenseSummary.
type ExpenseSummaryAmount struct {
	Amount string `json:"amount"`
	Code   string `json:"code"`
	Count  int    `json:"count"`
}

// ExpenseSummary is one bucket (e.g. "grand_total", "active", "archived") of
// the expense summaries report.
type ExpenseSummary struct {
	ID      string                 `json:"id"`
	Counts  int                    `json:"counts"`
	Amounts []ExpenseSummaryAmount `json:"amounts"`
}

type expenseSummariesEnvelope struct {
	Summaries []ExpenseSummary `json:"summaries"`
}

func expenseSummariesPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/expenses/summaries", acct), nil
}

// Summaries returns the account's expense totals bucketed by status
// (grand_total, active, archived).
//
// inventory: Expenses/Expense Summaries
func (s *ExpensesService) Summaries(ctx context.Context, acct AccountID) ([]ExpenseSummary, error) {
	path, err := expenseSummariesPath(acct)
	if err != nil {
		return nil, err
	}
	var env expenseSummariesEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env); err != nil {
		return nil, err
	}
	return env.Summaries, nil
}

func expenseVendorsPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/expenses/vendors", acct), nil
}

// Vendors returns the distinct vendor names used across the account's
// expenses, walking every page.
//
// The wire shape is {"vendors": [{"vendor": "..."}], "page", "pages",
// "per_page", "total"} -- a paginated list of objects, each wrapping a
// single free-text vendor name (CONFIRMED live 2026-09-02). The default
// page size the API applies is 15, so this walks the pages and returns the
// flattened list: the method promises "the account's vendors", and silently
// returning the first 15 would be a trap.
//
// inventory: Expenses/Expense Vendors
func (s *ExpensesService) Vendors(ctx context.Context, acct AccountID) ([]string, error) {
	path, err := expenseVendorsPath(acct)
	if err != nil {
		return nil, err
	}
	// Non-nil so an account with no vendors encodes as [] rather than
	// null: the MCP tool and the CLI command both json.Marshal this
	// return value straight onto the wire.
	vendors := []string{}
	for page := 1; ; page++ {
		var env struct {
			PageMeta
			Vendors []struct {
				Vendor string `json:"vendor"`
			} `json:"vendors"`
		}
		if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, PageNumber(page)); err != nil {
			return nil, err
		}
		for _, v := range env.Vendors {
			vendors = append(vendors, v.Vendor)
		}
		if len(env.Vendors) == 0 || env.Pages <= page {
			return vendors, nil
		}
	}
}

// ExpenseProfile is a recurring-expense template. There is no
// ExpenseProfilesService field on Client (the design spec does not name
// one), so this lone endpoint lives on ExpensesService.
type ExpenseProfile struct {
	ProfileID     int64  `json:"profileid"`
	Frequency     string `json:"frequency"`
	StartDate     Date   `json:"start_date"`
	EndDate       Date   `json:"end_date,omitzero"`
	NextIssueDate Date   `json:"next_issue_date,omitzero"`
	Amount        Money  `json:"amount"`
	Notes         string `json:"notes,omitempty"`
	Vendor        string `json:"vendor,omitempty"`
	CategoryID    int64  `json:"categoryid,omitempty"`
	ClientID      int64  `json:"clientid,omitempty"`
	StaffID       int64  `json:"staffid,omitempty"`
	IsCOGS        bool   `json:"is_cogs,omitempty"`
}

type expenseProfileEnvelope struct {
	ExpenseProfile ExpenseProfile `json:"expense_profile"`
}

// ExpenseProfileCreateRequest is the payload for CreateRecurring. Frequency,
// StartDate, and Amount are required by the API. CategoryID is *int64,
// matching ExpenseWriteRequest's doc comment on why (docs field table +
// example, not the Postman example's quoted value).
type ExpenseProfileCreateRequest struct {
	Frequency     string   `json:"frequency"`
	StartDate     Date     `json:"start_date"`
	EndDate       *Date    `json:"end_date,omitempty"`
	NextIssueDate *Date    `json:"next_issue_date,omitempty"`
	Amount        Money    `json:"amount"`
	Notes         string   `json:"notes,omitempty"`
	Vendor        string   `json:"vendor,omitempty"`
	CategoryID    *int64   `json:"categoryid,omitempty"`
	ClientID      *int64   `json:"clientid,omitempty"`
	StaffID       *int64   `json:"staffid,omitempty"`
	TaxName1      string   `json:"taxName1,omitempty"`
	TaxAmount1    *Money   `json:"taxAmount1,omitempty"`
	TaxPercent1   *float64 `json:"taxPercent1,omitempty"`
}

func expenseProfilesPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/expense_profiles/expense_profiles", acct), nil
}

// CreateRecurring creates a recurring-expense profile that FreshBooks will
// use to generate expenses on the given schedule. The Postman collection
// carries no example response for this request; the response shape here
// mirrors the request, wrapped in "expense_profile" the way every other
// accounting write in this API responds -- INFERRED, not confirmed live.
//
// inventory: Expenses/Create Recurring Expense
func (s *ExpensesService) CreateRecurring(ctx context.Context, acct AccountID, req *ExpenseProfileCreateRequest) (*ExpenseProfile, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Expenses.CreateRecurring needs a request")
	}
	path, err := expenseProfilesPath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		ExpenseProfile *ExpenseProfileCreateRequest `json:"expense_profile"`
	}{req}
	var env expenseProfileEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.ExpenseProfile, nil
}
