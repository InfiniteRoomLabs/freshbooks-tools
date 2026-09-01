package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// ExpensesService is the expenses resource.
type ExpensesService struct{ client *Client }

// ExpenseAttachment is a receipt image already uploaded via the images
// endpoint (batch d), referenced by its JWT.
type ExpenseAttachment struct {
	AttachmentID string `json:"attachmentid,omitempty"`
	ID           string `json:"id,omitempty"`
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
	// CategoryID files the expense under a category.
	CategoryID int64 `json:"categoryid,omitempty"`
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
	TaxAmount1  Money  `json:"taxAmount1,omitempty"`
	TaxAmount2  Money  `json:"taxAmount2,omitempty"`
	TaxPercent1 string `json:"taxPercent1,omitempty"`
	TaxPercent2 string `json:"taxPercent2,omitempty"`
	// MarkupPercent is the markup applied when this expense is billed
	// through to a client.
	MarkupPercent string `json:"markup_percent,omitempty"`
	// Attachment is the receipt image, when one was uploaded.
	Attachment *ExpenseAttachment `json:"attachment,omitempty"`
	// Updated is the account-local last-modified timestamp.
	Updated string `json:"updated,omitempty"`
	// VisState is the expense's visibility state.
	VisState VisState `json:"vis_state"`
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
// CategoryID is a string here, not an int, because the Postman examples
// send it that way (e.g. "categoryid": "65679") even though the API returns
// it as an int on read; this is INFERRED from the example, not documented.
type ExpenseWriteRequest struct {
	Amount        *Money                    `json:"amount,omitempty"`
	Date          *Date                     `json:"date,omitempty"`
	CategoryID    string                    `json:"categoryid,omitempty"`
	ClientID      *int64                    `json:"clientid,omitempty"`
	StaffID       *int64                    `json:"staffid,omitempty"`
	Vendor        string                    `json:"vendor,omitempty"`
	Notes         string                    `json:"notes,omitempty"`
	TaxName1      string                    `json:"taxName1,omitempty"`
	TaxName2      string                    `json:"taxName2,omitempty"`
	TaxAmount1    *Money                    `json:"taxAmount1,omitempty"`
	TaxAmount2    *Money                    `json:"taxAmount2,omitempty"`
	TaxPercent1   *float64                  `json:"taxPercent1,omitempty"`
	TaxPercent2   *float64                  `json:"taxPercent2,omitempty"`
	MarkupPercent *float64                  `json:"markup_percent,omitempty"`
	IsCOGS        *bool                     `json:"is_cogs,omitempty"`
	Attachment    *ExpenseAttachmentRequest `json:"attachment,omitempty"`
}

// ExpenseListOptions filters and paginates List.
type ExpenseListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *ExpenseListOptions) requestOptions() []RequestOption {
	if o == nil {
		return nil
	}
	var opts []RequestOption
	if o.Search != nil {
		opts = append(opts, o.Search)
	}
	if o.Page > 0 {
		opts = append(opts, PageNumber(o.Page))
	}
	if o.PerPage > 0 {
		opts = append(opts, PerPage(o.PerPage))
	}
	return opts
}

func expensesPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/expenses/expenses", acct)
}

func expensePath(acct AccountID, id int64) string {
	return fmt.Sprintf("/accounting/account/%s/expenses/expenses/%d", acct, id)
}

// List returns one page of expenses.
//
// inventory: Expenses/List Expenses
func (s *ExpensesService) List(ctx context.Context, acct AccountID, opts *ExpenseListOptions) (*Page[Expense], error) {
	var env expenseListEnvelope
	if err := s.client.do(ctx, http.MethodGet, expensesPath(acct), FamilyAccounting, nil, &env, opts.requestOptions()...); err != nil {
		return nil, err
	}
	return &Page[Expense]{Items: env.Expenses, Page: env.Page, Pages: env.Pages, PerPage: env.PerPage, Total: env.Total}, nil
}

// All walks every page of expenses, auto-paginating.
func (s *ExpensesService) All(ctx context.Context, acct AccountID, opts *ExpenseListOptions) iter.Seq2[Expense, error] {
	perPage := 100
	var search Search
	if opts != nil {
		if opts.PerPage > 0 {
			perPage = opts.PerPage
		}
		search = opts.Search
	}
	return All(ctx, func(ctx context.Context, page int) (*Page[Expense], error) {
		return s.List(ctx, acct, &ExpenseListOptions{Search: search, Page: page, PerPage: perPage})
	})
}

// Get retrieves a single expense.
//
// inventory: Expenses/Single Expense
func (s *ExpensesService) Get(ctx context.Context, acct AccountID, id int64) (*Expense, error) {
	var env expenseEnvelope
	if err := s.client.do(ctx, http.MethodGet, expensePath(acct, id), FamilyAccounting, nil, &env); err != nil {
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
	body := struct {
		Expense *ExpenseWriteRequest `json:"expense"`
	}{req}
	var env expenseEnvelope
	if err := s.client.do(ctx, http.MethodPost, expensesPath(acct), FamilyAccounting, body, &env); err != nil {
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
	body := struct {
		Expense *ExpenseWriteRequest `json:"expense"`
	}{req}
	var env expenseEnvelope
	if err := s.client.do(ctx, http.MethodPut, expensePath(acct, id), FamilyAccounting, body, &env); err != nil {
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
	body := map[string]any{"expense": map[string]any{"vis_state": VisStateDeleted}}
	return s.client.do(ctx, http.MethodPut, expensePath(acct, id), FamilyAccounting, body, nil)
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

// Summaries returns the account's expense totals bucketed by status
// (grand_total, active, archived).
//
// inventory: Expenses/Expense Summaries
func (s *ExpensesService) Summaries(ctx context.Context, acct AccountID) ([]ExpenseSummary, error) {
	var env expenseSummariesEnvelope
	path := fmt.Sprintf("/accounting/account/%s/expenses/summaries", acct)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env); err != nil {
		return nil, err
	}
	return env.Summaries, nil
}

// Vendors returns the distinct vendor names used across the account's
// expenses. The Postman collection carries no example response for this
// request; the shape here (a bare string list under "vendors") is INFERRED
// from the fact that Expense.Vendor is a free-text field, not a foreign key
// -- a Phase 2 live check should confirm it.
//
// inventory: Expenses/Expense Vendors
func (s *ExpensesService) Vendors(ctx context.Context, acct AccountID) ([]string, error) {
	var env struct {
		Vendors []string `json:"vendors"`
	}
	path := fmt.Sprintf("/accounting/account/%s/expenses/vendors", acct)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env); err != nil {
		return nil, err
	}
	return env.Vendors, nil
}

// ExpenseProfile is a recurring-expense template. There is no
// ExpenseProfilesService field on Client (the design spec does not name
// one), so this lone endpoint lives on ExpensesService.
type ExpenseProfile struct {
	ProfileID     int64  `json:"profileid"`
	Frequency     string `json:"frequency"`
	StartDate     Date   `json:"start_date"`
	EndDate       Date   `json:"end_date,omitempty"`
	NextIssueDate Date   `json:"next_issue_date,omitempty"`
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
// StartDate, and Amount are required by the API.
type ExpenseProfileCreateRequest struct {
	Frequency     string   `json:"frequency"`
	StartDate     Date     `json:"start_date"`
	EndDate       *Date    `json:"end_date,omitempty"`
	NextIssueDate *Date    `json:"next_issue_date,omitempty"`
	Amount        Money    `json:"amount"`
	Notes         string   `json:"notes,omitempty"`
	Vendor        string   `json:"vendor,omitempty"`
	CategoryID    string   `json:"categoryid,omitempty"`
	ClientID      *int64   `json:"clientid,omitempty"`
	StaffID       *int64   `json:"staffid,omitempty"`
	TaxName1      string   `json:"taxName1,omitempty"`
	TaxAmount1    *Money   `json:"taxAmount1,omitempty"`
	TaxPercent1   *float64 `json:"taxPercent1,omitempty"`
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
	body := struct {
		ExpenseProfile *ExpenseProfileCreateRequest `json:"expense_profile"`
	}{req}
	var env expenseProfileEnvelope
	path := fmt.Sprintf("/accounting/account/%s/expense_profiles/expense_profiles", acct)
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.ExpenseProfile, nil
}
