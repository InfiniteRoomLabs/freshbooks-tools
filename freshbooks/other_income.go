package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// OtherIncomeTax is one tax line applied to an other-income record.
type OtherIncomeTax struct {
	// Amount is the tax amount as a decimal string.
	Amount string `json:"amount"`
	// Name is the tax's display name, e.g. "HST".
	Name string `json:"name"`
}

// OtherIncome is a non-invoice income record: money received outside the
// normal invoicing flow (e.g. marketplace sales), recorded for accounting
// and reporting purposes.
type OtherIncome struct {
	// IncomeID identifies the record.
	IncomeID int64 `json:"incomeid"`
	// Amount is the income amount.
	Amount Money `json:"amount"`
	// CategoryName classifies the income, e.g. "online_sales".
	CategoryName string `json:"category_name"`
	// Date is when the income was received, "YYYY-MM-DD".
	Date string `json:"date"`
	// Note is a free-text description.
	Note string `json:"note"`
	// PaymentType names how the payment was received, e.g. "VISA".
	PaymentType string `json:"payment_type"`
	// Source names where the income came from, e.g. "Etsy".
	Source string `json:"source"`
	// Taxes are the tax lines applied to this income.
	Taxes []OtherIncomeTax `json:"taxes"`
	// CreatedAt and UpdatedAt are account-local timestamps
	// ("YYYY-MM-DD HH:MM:SS").
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	// VisState is the record's visibility state; VisStateDeleted marks it
	// soft-deleted.
	VisState VisState `json:"vis_state"`
}

// OtherIncomeCreateRequest is the payload for OtherIncomeService.Create.
type OtherIncomeCreateRequest struct {
	// Amount is the income amount.
	Amount Money `json:"amount"`
	// CategoryName classifies the income, e.g. "online_sales".
	CategoryName string `json:"category_name,omitempty"`
	// Date is when the income was received, "YYYY-MM-DD".
	Date string `json:"date"`
	// Note is a free-text description.
	Note string `json:"note,omitempty"`
	// PaymentType names how the payment was received.
	PaymentType string `json:"payment_type,omitempty"`
	// Source names where the income came from.
	Source string `json:"source,omitempty"`
	// Taxes are the tax lines to apply.
	Taxes []OtherIncomeTax `json:"taxes,omitempty"`
}

// OtherIncomeUpdateRequest is the payload for OtherIncomeService.Update. Every
// clearable scalar field is a pointer, one consistent partial-update
// convention: a nil field is left alone, a non-nil field (including a
// pointer to "") is sent and overwrites the existing value.
type OtherIncomeUpdateRequest struct {
	// Amount replaces the income amount when set.
	Amount *Money `json:"amount,omitempty"`
	// CategoryName replaces the income's category when set.
	CategoryName *string `json:"category_name,omitempty"`
	// Date replaces the income date ("YYYY-MM-DD") when set.
	Date *string `json:"date,omitempty"`
	// Note replaces the free-text description when set.
	Note *string `json:"note,omitempty"`
	// PaymentType replaces how the payment was received when set.
	PaymentType *string `json:"payment_type,omitempty"`
	// Source replaces where the income came from when set.
	Source *string `json:"source,omitempty"`
	// Taxes replaces the tax lines when set.
	Taxes []OtherIncomeTax `json:"taxes,omitempty"`
	// VisState is set by Delete to soft-delete the record; leave nil for a
	// plain field update.
	VisState *VisState `json:"vis_state,omitempty"`
}

// otherIncomeEnvelope is the {"other_income": {...}} shape every
// non-list other-income response nests its record in.
type otherIncomeEnvelope struct {
	OtherIncome OtherIncome `json:"other_income"`
}

// otherIncomePath validates acct and builds the other-income collection
// path. Postman lists these operations twice, under Accounting/Other Income
// and Invoices/Other Income; both sets of names map to the methods below.
func otherIncomePath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return "/accounting/account/" + string(acct) + "/other_incomes/other_incomes", nil
}

// otherIncomeItemPath validates acct and builds one record's item path.
func otherIncomeItemPath(acct AccountID, incomeID int64) (string, error) {
	base, err := otherIncomePath(acct)
	if err != nil {
		return "", err
	}
	return base + "/" + strconv.FormatInt(incomeID, 10), nil
}

// Create records a new other-income entry.
//
// inventory: Accounting/Other Income/Create Single Other Income
// inventory: Invoices/Other Income/Create Single Other Income
func (s *OtherIncomeService) Create(ctx context.Context, acct AccountID, req *OtherIncomeCreateRequest) (*OtherIncome, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: OtherIncome.Create needs a request")
	}
	path, err := otherIncomePath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		OtherIncome *OtherIncomeCreateRequest `json:"other_income"`
	}{req}
	var resp otherIncomeEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.OtherIncome, nil
}

// OtherIncomeListOptions filters and paginates List.
type OtherIncomeListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *OtherIncomeListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

// List returns one page of acct's other-income records.
//
// inventory: Accounting/Other Income/List Other Income
// inventory: Invoices/Other Income/List Other Income
func (s *OtherIncomeService) List(ctx context.Context, acct AccountID, opts *OtherIncomeListOptions, extra ...RequestOption) (*Page[OtherIncome], error) {
	path, err := otherIncomePath(acct)
	if err != nil {
		return nil, err
	}
	var resp struct {
		OtherIncome []OtherIncome `json:"other_income"`
		PageMeta
	}
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.OtherIncome, resp.PageMeta), nil
}

// All walks every page of List.
func (s *OtherIncomeService) All(ctx context.Context, acct AccountID, opts *OtherIncomeListOptions, extra ...RequestOption) iter.Seq2[OtherIncome, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[OtherIncome], error) {
		o := OtherIncomeListOptions{}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		o.PerPage = pageSize(o.PerPage)
		pageOpts := append(append([]RequestOption{}, extra...), PageNumber(page))
		return s.List(ctx, acct, &o, pageOpts...)
	})
}

// Update changes fields on an existing other-income record.
//
// inventory: Accounting/Other Income/Update Single Other Income
// inventory: Invoices/Other Income/Update Single Other Income
func (s *OtherIncomeService) Update(ctx context.Context, acct AccountID, incomeID int64, req *OtherIncomeUpdateRequest) (*OtherIncome, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: OtherIncome.Update needs a request")
	}
	path, err := otherIncomeItemPath(acct, incomeID)
	if err != nil {
		return nil, err
	}
	body := struct {
		OtherIncome *OtherIncomeUpdateRequest `json:"other_income"`
	}{req}
	var resp otherIncomeEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.OtherIncome, nil
}

// Delete soft-deletes an other-income record. FreshBooks has no DELETE verb
// for this resource; deletion is a PUT that sets vis_state to
// VisStateDeleted, the same endpoint Update uses for a field change --
// Delete just supplies a canned body. It stays on Update rather than the
// shared softDelete helper because the API returns the updated record and
// softDelete discards it.
//
// inventory: Accounting/Other Income/Delete Single Other Income
// inventory: Invoices/Other Income/Delete Single Other Income
func (s *OtherIncomeService) Delete(ctx context.Context, acct AccountID, incomeID int64) (*OtherIncome, error) {
	deleted := VisStateDeleted
	return s.Update(ctx, acct, incomeID, &OtherIncomeUpdateRequest{VisState: &deleted})
}
