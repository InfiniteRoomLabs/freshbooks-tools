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
	UnitCost      Money  `json:"unit_cost,omitzero"`
	Amount        Money  `json:"amount,omitzero"`
	CategoryID    int64  `json:"categoryid,omitempty"`
	TaxName1      string `json:"tax_name1,omitempty"`
	TaxName2      string `json:"tax_name2,omitempty"`
	TaxPercent1   *int   `json:"tax_percent1,omitempty"`
	TaxPercent2   *int   `json:"tax_percent2,omitempty"`
	TaxAmount1    Money  `json:"tax_amount1,omitzero"`
	TaxAmount2    Money  `json:"tax_amount2,omitzero"`
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
	DueDate   Date `json:"due_date,omitzero"`
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
	Amount      Money `json:"amount,omitzero"`
	TotalAmount Money `json:"total_amount,omitzero"`
	Outstanding Money `json:"outstanding,omitzero"`
	Paid        Money `json:"paid,omitzero"`
	TaxAmount   Money `json:"tax_amount,omitzero"`
	// Lines are the bill's line items.
	Lines []BillLine `json:"lines,omitempty"`
	// BillPayments are the payments recorded against this bill.
	BillPayments []BillPayment `json:"bill_payments,omitempty"`
	// CreatedAt and UpdatedAt are account-local timestamps.
	CreatedAt DateTime `json:"created_at,omitempty"`
	UpdatedAt DateTime `json:"updated_at,omitempty"`
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
	Description string `json:"description,omitempty"`
	Quantity    int    `json:"quantity,omitempty"`
	UnitCost    Money  `json:"unit_cost,omitzero"`
	CategoryID  int64  `json:"categoryid,omitempty"`
	TaxName1    string `json:"tax_name1,omitempty"`
	TaxName2    string `json:"tax_name2,omitempty"`
	TaxPercent1 *int   `json:"tax_percent1,omitempty"`
	TaxPercent2 *int   `json:"tax_percent2,omitempty"`
	// CompoundedTax is a pointer so a caller can explicitly send false, not
	// just omit the field to leave it unset.
	CompoundedTax *bool `json:"compounded_tax,omitempty"`
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

func (o *BillListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

func billsPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/bills/bills", acct), nil
}

func billPath(acct AccountID, id int64) (string, error) {
	base, err := billsPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}

// List returns one page of bills.
//
// inventory: Expenses/Bills (Beta)/Get Bills
func (s *BillsService) List(ctx context.Context, acct AccountID, opts *BillListOptions, extra ...RequestOption) (*Page[Bill], error) {
	path, err := billsPath(acct)
	if err != nil {
		return nil, err
	}
	var env billListEnvelope
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(env.Bills, env.PageMeta), nil
}

// All walks every page of bills, auto-paginating.
func (s *BillsService) All(ctx context.Context, acct AccountID, opts *BillListOptions, extra ...RequestOption) iter.Seq2[Bill, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Bill], error) {
		o := BillListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		o.PerPage = pageSize(o.PerPage)
		return s.List(ctx, acct, &o, extra...)
	})
}

// Create adds a new bill from a vendor.
//
// inventory: Expenses/Bills (Beta)/Add Bill from Vendor
func (s *BillsService) Create(ctx context.Context, acct AccountID, req *BillCreateRequest) (*Bill, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Bills.Create needs a request")
	}
	path, err := billsPath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		Bill *BillCreateRequest `json:"bill"`
	}{req}
	var env billEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Bill, nil
}

// visStatePut sends {"bill": {"vis_state": state}} to id's bill and decodes
// the updated bill back. Kept distinct from (*Client).softDelete: Archive
// needs a state softDelete does not support, and Delete's caller wants the
// decode-error visibility of a non-nil out, so both share this local helper
// instead of half of them switching to softDelete.
func (s *BillsService) visStatePut(ctx context.Context, acct AccountID, id int64, state VisState) (*Bill, error) {
	path, err := billPath(acct, id)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"bill": map[string]any{"vis_state": state}}
	var env billEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &env); err != nil {
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
