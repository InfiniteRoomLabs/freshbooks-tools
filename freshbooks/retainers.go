package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// RetainersService (declared in services.go) is the business-scoped
// retainers resource: a recurring minimum-fee arrangement with a client,
// billed automatically each period. Unlike every other service in this
// batch, retainers live under /comments/business/{businessId}/...,
// addressed by BusinessID rather than AccountID: FreshBooks' flat,
// meta-paginated business family, not the enveloped accounting one.

// Retainer is a recurring minimum-fee arrangement with a client.
type Retainer struct {
	ID                  int64  `json:"id"`
	BusinessID          int64  `json:"business_id"`
	SystemID            int64  `json:"system_id"`
	InvoiceProfileID    *int64 `json:"invoice_profile_id,omitempty"`
	ClientUserID        int64  `json:"client_user_id"`
	StartDate           string `json:"start_date"`
	NextPeriodStartDate string `json:"next_period_start_date,omitempty"`
	Fee                 string `json:"fee"`
	ExcessRate          string `json:"excess_rate,omitempty"`
	BudgetedTime        int64  `json:"budgeted_time,omitempty"`
	Active              bool   `json:"active"`
	CreatedAt           string `json:"created_at,omitempty"`
	UpdatedAt           string `json:"updated_at,omitempty"`
}

type retainerEnvelope struct {
	Retainer *Retainer `json:"retainer"`
}

type retainerWriteEnvelope struct {
	Retainer any `json:"retainer"`
}

type retainerListResponse struct {
	Retainers []Retainer `json:"retainers"`
	Meta      PageMeta   `json:"meta"`
}

// RetainerListOptions selects and paginates List. The business family's
// filter encoding is bare field=value, INFERRED from the FreshBooks docs
// (see spec section 5.1's STATE AS OF callout); this is the first
// business-scoped list endpoint this library implements against it.
type RetainerListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *RetainerListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	var opts []RequestOption
	if len(o.Search) > 0 {
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

// List returns all retainers for a business. FreshBooks' Postman example
// response has no "meta" pagination block for this endpoint (unlike the
// accounting family, which always includes one); Meta on the returned page
// is therefore zero-valued unless the live API sends it.
//
// inventory: Invoices/Retainers/Get all retainers
func (s *RetainersService) List(ctx context.Context, businessID BusinessID, opts *RetainerListOptions, extra ...RequestOption) (*Page[Retainer], error) {
	var resp retainerListResponse
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, retainersPath(businessID), FamilyBusiness, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return &Page[Retainer]{Items: resp.Retainers, Page: resp.Meta.Page, Pages: resp.Meta.Pages, PerPage: resp.Meta.PerPage, Total: resp.Meta.Total}, nil
}

// Get fetches one retainer by id.
//
// inventory: Invoices/Retainers/Single Retainer
func (s *RetainersService) Get(ctx context.Context, businessID BusinessID, id int64) (*Retainer, error) {
	var resp retainerEnvelope
	if err := s.client.do(ctx, http.MethodGet, retainerPath(businessID, id), FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Retainer, nil
}

// RetainerCreateRequest is the payload for Create.
type RetainerCreateRequest struct {
	ClientUserID          string  `json:"client_user_id"`
	StartDate             string  `json:"start_date"`
	NextPeriodStartDate   string  `json:"next_period_start_date,omitempty"`
	Fee                   float64 `json:"fee"`
	ExcessRate            float64 `json:"excess_rate,omitempty"`
	BudgetedTime          int64   `json:"budgeted_time,omitempty"`
	Active                bool    `json:"active"`
	Frequency             string  `json:"frequency,omitempty"`
	NumberRecurring       int     `json:"number_recurring,omitempty"`
	IsInfinitelyRecurring bool    `json:"is_infinitely_recurring,omitempty"`
}

// Create starts a new retainer for a client. FreshBooks allows only one
// active retainer per client at a time.
//
// inventory: Invoices/Retainers/Create Retainer
func (s *RetainersService) Create(ctx context.Context, businessID BusinessID, req *RetainerCreateRequest) (*Retainer, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Retainers.Create needs a request")
	}
	var resp retainerEnvelope
	if err := s.client.do(ctx, http.MethodPost, retainersPath(businessID), FamilyBusiness, retainerWriteEnvelope{Retainer: req}, &resp); err != nil {
		return nil, err
	}
	return resp.Retainer, nil
}

// RetainerUpdateRequest is the payload for Update.
type RetainerUpdateRequest struct {
	ClientUserID          string  `json:"client_user_id,omitempty"`
	StartDate             string  `json:"start_date,omitempty"`
	NextPeriodStartDate   string  `json:"next_period_start_date,omitempty"`
	Fee                   float64 `json:"fee,omitempty"`
	ExcessRate            float64 `json:"excess_rate,omitempty"`
	BudgetedTime          int64   `json:"budgeted_time,omitempty"`
	Active                bool    `json:"active"`
	Frequency             string  `json:"frequency,omitempty"`
	NumberRecurring       int     `json:"number_recurring,omitempty"`
	IsInfinitelyRecurring bool    `json:"is_infinitely_recurring,omitempty"`
}

// Update replaces a retainer's terms.
//
// inventory: Invoices/Retainers/Update Retainer
func (s *RetainersService) Update(ctx context.Context, businessID BusinessID, id int64, req *RetainerUpdateRequest) (*Retainer, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Retainers.Update needs a request")
	}
	var resp retainerEnvelope
	if err := s.client.do(ctx, http.MethodPut, retainerPath(businessID, id), FamilyBusiness, retainerWriteEnvelope{Retainer: req}, &resp); err != nil {
		return nil, err
	}
	return resp.Retainer, nil
}

// Delete deactivates a retainer. FreshBooks has no hard delete for
// retainers; this sets active: false, the same soft-delete shape the
// accounting family expresses through vis_state.
//
// inventory: Invoices/Retainers/Delete Retainer
func (s *RetainersService) Delete(ctx context.Context, businessID BusinessID, id int64) (*Retainer, error) {
	var resp retainerEnvelope
	body := retainerWriteEnvelope{Retainer: map[string]bool{"active": false}}
	if err := s.client.do(ctx, http.MethodPut, retainerPath(businessID, id), FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return resp.Retainer, nil
}

// Undelete reactivates a previously deleted retainer.
//
// inventory: Invoices/Retainers/Undelete Retainer
func (s *RetainersService) Undelete(ctx context.Context, businessID BusinessID, id int64) (*Retainer, error) {
	var resp retainerEnvelope
	body := retainerWriteEnvelope{Retainer: map[string]bool{"active": true}}
	if err := s.client.do(ctx, http.MethodPut, retainerPath(businessID, id), FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return resp.Retainer, nil
}

func retainersPath(businessID BusinessID) string {
	return fmt.Sprintf("/comments/business/%s/retainers", businessID)
}

func retainerPath(businessID BusinessID, id int64) string {
	return fmt.Sprintf("/comments/business/%s/retainer/%d", businessID, id)
}
