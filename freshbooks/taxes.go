package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// TaxesService is the tax-rates resource. The Postman collection lists the
// same five operations three times, once per menu they appear under in the
// FreshBooks UI (Expenses, Accounting, and Settings > Items and Services);
// every method here carries all three inventory keys, following the
// precedent set by IdentityService.Me.
type TaxesService struct{ client *Client }

// Tax is a tax rate.
type Tax struct {
	// ID and TaxID are the same value under two names; the API returns both.
	ID    int64 `json:"id"`
	TaxID int64 `json:"taxid"`
	// AccountingSystemID is the account this tax belongs to.
	AccountingSystemID string `json:"accounting_systemid,omitempty"`
	// Name is the tax's display name, e.g. "HST".
	Name string `json:"name"`
	// Number is an optional tax registration number.
	Number string `json:"number,omitempty"`
	// Amount is the tax rate as a decimal string, e.g. "13".
	Amount string `json:"amount"`
	// Compound reports whether this tax compounds on top of another.
	Compound bool `json:"compound,omitempty"`
	// Updated is the account-local last-modified timestamp.
	Updated string `json:"updated,omitempty"`
}

type taxEnvelope struct {
	Tax Tax `json:"tax"`
}

type taxListEnvelope struct {
	Taxes []Tax `json:"taxes"`
	PageMeta
}

// TaxCreateRequest is the payload for Create. Name is required; the API
// accepts Amount as a bare JSON number, not a Money object.
type TaxCreateRequest struct {
	Name   string   `json:"name"`
	Number *string  `json:"number,omitempty"`
	Amount *float64 `json:"amount,omitempty"`
}

// TaxUpdateRequest is the payload for Update. Only non-nil fields are sent.
type TaxUpdateRequest struct {
	Name     *string  `json:"name,omitempty"`
	Number   *string  `json:"number,omitempty"`
	Amount   *float64 `json:"amount,omitempty"`
	Compound *bool    `json:"compound,omitempty"`
}

// TaxListOptions filters and paginates List.
type TaxListOptions struct {
	// Search filters the list, e.g. Search{"name": "HST"}.
	Search Search
	// Page selects a 1-based page.
	Page int
	// PerPage sets the page size.
	PerPage int
}

func (o *TaxListOptions) requestOptions() []RequestOption {
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

func taxesPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/taxes/taxes", acct)
}

func taxPath(acct AccountID, taxID int64) string {
	return fmt.Sprintf("/accounting/account/%s/taxes/taxes/%d", acct, taxID)
}

// List returns one page of tax rates.
//
// inventory: Expenses/List Taxes
// inventory: Accounting/Taxes/List Taxes
// inventory: Settings/Items and Services/List Taxes
func (s *TaxesService) List(ctx context.Context, acct AccountID, opts *TaxListOptions) (*Page[Tax], error) {
	var env taxListEnvelope
	if err := s.client.do(ctx, http.MethodGet, taxesPath(acct), FamilyAccounting, nil, &env, opts.requestOptions()...); err != nil {
		return nil, err
	}
	return &Page[Tax]{Items: env.Taxes, Page: env.Page, Pages: env.Pages, PerPage: env.PerPage, Total: env.Total}, nil
}

// All walks every page of tax rates, auto-paginating.
func (s *TaxesService) All(ctx context.Context, acct AccountID, opts *TaxListOptions) iter.Seq2[Tax, error] {
	perPage := 100
	var search Search
	if opts != nil {
		if opts.PerPage > 0 {
			perPage = opts.PerPage
		}
		search = opts.Search
	}
	return All(ctx, func(ctx context.Context, page int) (*Page[Tax], error) {
		return s.List(ctx, acct, &TaxListOptions{Search: search, Page: page, PerPage: perPage})
	})
}

// Get retrieves a single tax rate.
//
// inventory: Expenses/Single Tax (GET)
// inventory: Accounting/Taxes/Get Single Tax
// inventory: Settings/Items and Services/Single Tax (GET)
func (s *TaxesService) Get(ctx context.Context, acct AccountID, taxID int64) (*Tax, error) {
	var env taxEnvelope
	if err := s.client.do(ctx, http.MethodGet, taxPath(acct, taxID), FamilyAccounting, nil, &env); err != nil {
		return nil, err
	}
	return &env.Tax, nil
}

// Create adds a new tax rate.
//
// inventory: Expenses/Create Single Tax
// inventory: Accounting/Taxes/Create Single Tax
// inventory: Settings/Items and Services/Create Single Tax
func (s *TaxesService) Create(ctx context.Context, acct AccountID, req *TaxCreateRequest) (*Tax, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Taxes.Create needs a request")
	}
	body := struct {
		Tax *TaxCreateRequest `json:"tax"`
	}{req}
	var env taxEnvelope
	if err := s.client.do(ctx, http.MethodPost, taxesPath(acct), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Tax, nil
}

// Update changes an existing tax rate. Only non-nil fields on req are sent.
//
// inventory: Expenses/Update Tax
// inventory: Accounting/Taxes/Update Single Tax
// inventory: Settings/Items and Services/Update Tax
func (s *TaxesService) Update(ctx context.Context, acct AccountID, taxID int64, req *TaxUpdateRequest) (*Tax, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Taxes.Update needs a request")
	}
	body := struct {
		Tax *TaxUpdateRequest `json:"tax"`
	}{req}
	var env taxEnvelope
	if err := s.client.do(ctx, http.MethodPut, taxPath(acct, taxID), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Tax, nil
}

// Delete permanently removes a tax rate. Unlike expenses, estimates, bills,
// and credit notes, taxes have a real HTTP DELETE rather than a
// vis_state-flagged PUT.
//
// inventory: Expenses/Single Tax (DELETE)
// inventory: Accounting/Taxes/Delete Single Tax
// inventory: Settings/Items and Services/Single Tax (DELETE)
func (s *TaxesService) Delete(ctx context.Context, acct AccountID, taxID int64) error {
	return s.client.do(ctx, http.MethodDelete, taxPath(acct, taxID), FamilyAccounting, nil, nil)
}
