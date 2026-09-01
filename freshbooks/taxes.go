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
	Updated DateTime `json:"updated,omitempty"`
}

type taxEnvelope struct {
	Tax Tax `json:"tax"`
}

type taxListEnvelope struct {
	Taxes []Tax `json:"taxes"`
	PageMeta
}

// TaxCreateRequest is the payload for Create. Name is required; the API
// accepts Amount as a bare JSON number, not a Money object. Compound lets a
// caller create a compound tax in one call, matching TaxUpdateRequest and
// the Tax read model.
type TaxCreateRequest struct {
	Name     string   `json:"name"`
	Number   *string  `json:"number,omitempty"`
	Amount   *float64 `json:"amount,omitempty"`
	Compound *bool    `json:"compound,omitempty"`
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

func (o *TaxListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

func taxesPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/taxes/taxes", acct), nil
}

func taxPath(acct AccountID, taxID int64) (string, error) {
	base, err := taxesPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, taxID), nil
}

// List returns one page of tax rates.
//
// inventory: Expenses/List Taxes
// inventory: Accounting/Taxes/List Taxes
// inventory: Settings/Items and Services/List Taxes
func (s *TaxesService) List(ctx context.Context, acct AccountID, opts *TaxListOptions, extra ...RequestOption) (*Page[Tax], error) {
	path, err := taxesPath(acct)
	if err != nil {
		return nil, err
	}
	var env taxListEnvelope
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(env.Taxes, env.PageMeta), nil
}

// All walks every page of tax rates, auto-paginating.
func (s *TaxesService) All(ctx context.Context, acct AccountID, opts *TaxListOptions, extra ...RequestOption) iter.Seq2[Tax, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Tax], error) {
		o := TaxListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		o.PerPage = pageSize(o.PerPage)
		return s.List(ctx, acct, &o, extra...)
	})
}

// Get retrieves a single tax rate.
//
// inventory: Expenses/Single Tax (GET)
// inventory: Accounting/Taxes/Get Single Tax
// inventory: Settings/Items and Services/Single Tax (GET)
func (s *TaxesService) Get(ctx context.Context, acct AccountID, taxID int64, opts ...RequestOption) (*Tax, error) {
	path, err := taxPath(acct, taxID)
	if err != nil {
		return nil, err
	}
	var env taxEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, opts...); err != nil {
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
	path, err := taxesPath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		Tax *TaxCreateRequest `json:"tax"`
	}{req}
	var env taxEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
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
	path, err := taxPath(acct, taxID)
	if err != nil {
		return nil, err
	}
	body := struct {
		Tax *TaxUpdateRequest `json:"tax"`
	}{req}
	var env taxEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &env); err != nil {
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
	path, err := taxPath(acct, taxID)
	if err != nil {
		return err
	}
	return s.client.do(ctx, http.MethodDelete, path, FamilyAccounting, nil, nil)
}
