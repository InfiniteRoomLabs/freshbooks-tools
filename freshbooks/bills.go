package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// BillsService is the vendor-bills resource. Bills are a Beta endpoint in
// the Postman collection ("Bills (Beta)"); facts here are INFERRED from the
// collection's examples and the FreshBooks docs page, not live-verified.
type BillsService struct{ client *Client }

// BillLine is one line item on a bill.
type BillLine struct {
	Description   string `json:"description,omitempty"`
	Quantity      int    `json:"quantity,omitempty"`
	UnitCost      Money  `json:"unit_cost,omitempty"`
	Amount        Money  `json:"amount,omitempty"`
	CategoryID    int64  `json:"categoryid,omitempty"`
	TaxName1      string `json:"tax_name1,omitempty"`
	TaxName2      string `json:"tax_name2,omitempty"`
	TaxPercent1   *int   `json:"tax_percent1,omitempty"`
	TaxPercent2   *int   `json:"tax_percent2,omitempty"`
	CompoundedTax bool   `json:"compounded_tax,omitempty"`
}

// Bill is a vendor bill.
type Bill struct {
	// ID is the bill's identifier.
	ID int64 `json:"id"`
	// VendorID is the vendor this bill was raised against.
	VendorID int64 `json:"vendorid,omitempty"`
	// BillNumber is an optional vendor-assigned reference number.
	BillNumber string `json:"bill_number,omitempty"`
	// IssueDate and DueDate bound the bill.
	IssueDate Date `json:"issue_date"`
	DueDate   Date `json:"due_date,omitempty"`
	// DueOffsetDays is the number of days after IssueDate the bill is due.
	DueOffsetDays int `json:"due_offset_days,omitempty"`
	// CurrencyCode is the bill's ISO 4217 currency.
	CurrencyCode string `json:"currency_code,omitempty"`
	// Language is the bill's language code.
	Language string `json:"language,omitempty"`
	// Status is the bill's payment status: "unpaid", "overdue", "partial",
	// or "paid".
	Status string `json:"status,omitempty"`
	// Amount, TotalAmount, Outstanding, and Paid summarize the bill's
	// balances.
	Amount      Money `json:"amount,omitempty"`
	TotalAmount Money `json:"total_amount,omitempty"`
	Outstanding Money `json:"outstanding,omitempty"`
	Paid        Money `json:"paid,omitempty"`
	TaxAmount   Money `json:"tax_amount,omitempty"`
	// Lines are the bill's line items.
	Lines []BillLine `json:"lines,omitempty"`
	// BillPayments are the payments recorded against this bill.
	BillPayments []BillPayment `json:"bill_payments,omitempty"`
	// CreatedAt and UpdatedAt are account-local timestamps.
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	// VisState is the bill's visibility state (0 active, 1 deleted, 2
	// archived).
	VisState VisState `json:"vis_state"`
}

type billEnvelope struct {
	Bill Bill `json:"bill"`
}

type billListEnvelope struct {
	Bills []Bill `json:"bills"`
	PageMeta
}

// BillLineRequest is one line item in a BillCreateRequest.
type BillLineRequest struct {
	Description   string `json:"description,omitempty"`
	Quantity      int    `json:"quantity,omitempty"`
	UnitCost      Money  `json:"unit_cost,omitempty"`
	CategoryID    int64  `json:"categoryid,omitempty"`
	TaxName1      string `json:"tax_name1,omitempty"`
	TaxName2      string `json:"tax_name2,omitempty"`
	TaxPercent1   *int   `json:"tax_percent1,omitempty"`
	TaxPercent2   *int   `json:"tax_percent2,omitempty"`
	CompoundedTax bool   `json:"compounded_tax,omitempty"`
}

// BillCreateRequest is the payload for Create. VendorID and IssueDate are
// required by the API.
type BillCreateRequest struct {
	VendorID      int64             `json:"vendorid"`
	BillNumber    string            `json:"bill_number,omitempty"`
	IssueDate     Date              `json:"issue_date"`
	DueOffsetDays int               `json:"due_offset_days,omitempty"`
	CurrencyCode  string            `json:"currency_code,omitempty"`
	Language      string            `json:"language,omitempty"`
	Lines         []BillLineRequest `json:"lines,omitempty"`
}

// BillListOptions filters and paginates List.
type BillListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *BillListOptions) requestOptions() []RequestOption {
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

func billsPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/bills/bills", acct)
}

func billPath(acct AccountID, id int64) string {
	return fmt.Sprintf("/accounting/account/%s/bills/bills/%d", acct, id)
}

// List returns one page of bills.
//
// inventory: Expenses/Bills (Beta)/Get Bills
func (s *BillsService) List(ctx context.Context, acct AccountID, opts *BillListOptions) (*Page[Bill], error) {
	var env billListEnvelope
	if err := s.client.do(ctx, http.MethodGet, billsPath(acct), FamilyAccounting, nil, &env, opts.requestOptions()...); err != nil {
		return nil, err
	}
	return &Page[Bill]{Items: env.Bills, Page: env.Page, Pages: env.Pages, PerPage: env.PerPage, Total: env.Total}, nil
}

// All walks every page of bills, auto-paginating.
func (s *BillsService) All(ctx context.Context, acct AccountID, opts *BillListOptions) iter.Seq2[Bill, error] {
	perPage := 100
	var search Search
	if opts != nil {
		if opts.PerPage > 0 {
			perPage = opts.PerPage
		}
		search = opts.Search
	}
	return All(ctx, func(ctx context.Context, page int) (*Page[Bill], error) {
		return s.List(ctx, acct, &BillListOptions{Search: search, Page: page, PerPage: perPage})
	})
}

// Create adds a new bill from a vendor.
//
// inventory: Expenses/Bills (Beta)/Add Bill from Vendor
func (s *BillsService) Create(ctx context.Context, acct AccountID, req *BillCreateRequest) (*Bill, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Bills.Create needs a request")
	}
	body := struct {
		Bill *BillCreateRequest `json:"bill"`
	}{req}
	var env billEnvelope
	if err := s.client.do(ctx, http.MethodPost, billsPath(acct), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Bill, nil
}

// visStatePut sends {"bill": {"vis_state": state}} to id's bill and decodes
// the updated bill back.
func (s *BillsService) visStatePut(ctx context.Context, acct AccountID, id int64, state VisState) (*Bill, error) {
	body := map[string]any{"bill": map[string]any{"vis_state": state}}
	var env billEnvelope
	if err := s.client.do(ctx, http.MethodPut, billPath(acct, id), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Bill, nil
}

// Archive marks a bill archived (vis_state 2). FreshBooks models this as a
// PUT, not a distinct endpoint.
//
// inventory: Expenses/Bills (Beta)/Archive Bill
func (s *BillsService) Archive(ctx context.Context, acct AccountID, id int64) (*Bill, error) {
	return s.visStatePut(ctx, acct, id, VisStateArchived)
}

// Delete soft-deletes a bill (vis_state 1). FreshBooks models this as a PUT,
// not a real HTTP DELETE; consistent with the rest of the library's Delete
// vocabulary, the updated record is discarded and only the error matters.
//
// inventory: Expenses/Bills (Beta)/Delete Bill
func (s *BillsService) Delete(ctx context.Context, acct AccountID, id int64) error {
	_, err := s.visStatePut(ctx, acct, id, VisStateDeleted)
	return err
}
