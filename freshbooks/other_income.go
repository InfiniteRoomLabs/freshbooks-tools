package freshbooks

import (
	"context"
	"fmt"
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

// OtherIncomeUpdateRequest is the payload for OtherIncomeService.Update. All
// fields are optional; FreshBooks merges them onto the existing record.
type OtherIncomeUpdateRequest struct {
	Amount       *Money           `json:"amount,omitempty"`
	CategoryName string           `json:"category_name,omitempty"`
	Date         string           `json:"date,omitempty"`
	Note         *string          `json:"note,omitempty"`
	PaymentType  string           `json:"payment_type,omitempty"`
	Source       string           `json:"source,omitempty"`
	Taxes        []OtherIncomeTax `json:"taxes,omitempty"`
	// VisState is set by Delete to soft-delete the record; leave nil for a
	// plain field update.
	VisState *VisState `json:"vis_state,omitempty"`
}

// otherIncomeEnvelope is the {"other_income": {...}} shape every
// non-list other-income response nests its record in.
type otherIncomeEnvelope struct {
	OtherIncome OtherIncome `json:"other_income"`
}

// otherIncomePath builds the collection or item path for acct. Postman
// lists these operations twice, under Accounting/Other Income and
// Invoices/Other Income; both sets of names map to the methods below.
func otherIncomePath(acct AccountID, incomeID *int64) string {
	path := "/accounting/account/" + string(acct) + "/other_incomes/other_incomes"
	if incomeID != nil {
		path += "/" + strconv.FormatInt(*incomeID, 10)
	}
	return path
}

// Create records a new other-income entry.
//
// inventory: Accounting/Other Income/Create Single Other Income
// inventory: Invoices/Other Income/Create Single Other Income
func (s *OtherIncomeService) Create(ctx context.Context, acct AccountID, req *OtherIncomeCreateRequest) (*OtherIncome, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: OtherIncome.Create needs a request")
	}
	body := struct {
		OtherIncome *OtherIncomeCreateRequest `json:"other_income"`
	}{req}
	var resp otherIncomeEnvelope
	if err := s.client.do(ctx, http.MethodPost, otherIncomePath(acct, nil), FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.OtherIncome, nil
}

// List returns acct's other-income records, newest activity first.
//
// inventory: Accounting/Other Income/List Other Income
// inventory: Invoices/Other Income/List Other Income
func (s *OtherIncomeService) List(ctx context.Context, acct AccountID, opts ...RequestOption) (*Page[OtherIncome], error) {
	var resp struct {
		OtherIncome []OtherIncome `json:"other_income"`
		PageMeta
	}
	if err := s.client.do(ctx, http.MethodGet, otherIncomePath(acct, nil), FamilyAccounting, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &Page[OtherIncome]{
		Items: resp.OtherIncome, Page: resp.Page, Pages: resp.Pages,
		PerPage: resp.PerPage, Total: resp.Total,
	}, nil
}

// Update changes fields on an existing other-income record.
//
// inventory: Accounting/Other Income/Update Single Other Income
// inventory: Invoices/Other Income/Update Single Other Income
func (s *OtherIncomeService) Update(ctx context.Context, acct AccountID, incomeID int64, req *OtherIncomeUpdateRequest) (*OtherIncome, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: OtherIncome.Update needs a request")
	}
	body := struct {
		OtherIncome *OtherIncomeUpdateRequest `json:"other_income"`
	}{req}
	var resp otherIncomeEnvelope
	if err := s.client.do(ctx, http.MethodPut, otherIncomePath(acct, &incomeID), FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.OtherIncome, nil
}

// Delete soft-deletes an other-income record. FreshBooks has no DELETE verb
// for this resource; deletion is a PUT that sets vis_state to
// VisStateDeleted, the same endpoint Update uses for a field change --
// Delete just supplies a canned body.
//
// inventory: Accounting/Other Income/Delete Single Other Income
// inventory: Invoices/Other Income/Delete Single Other Income
func (s *OtherIncomeService) Delete(ctx context.Context, acct AccountID, incomeID int64) (*OtherIncome, error) {
	deleted := VisStateDeleted
	return s.Update(ctx, acct, incomeID, &OtherIncomeUpdateRequest{VisState: &deleted})
}
