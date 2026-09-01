package freshbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// RetainersService (declared in services.go) is the business-scoped
// retainers resource: a recurring minimum-fee arrangement with a client,
// billed automatically each period. Unlike every other service in this
// batch, retainers live under /comments/business/{businessId}/...,
// addressed by BusinessID rather than AccountID: FreshBooks' flat,
// meta-paginated business family, not the enveloped accounting one.
// BusinessID is a typed int64 whose String() only ever emits decimal
// digits (types.go), so unlike AccountID it needs no path-segment
// validation before interpolation.

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

// RetainerListOptions selects List. The business family's filter encoding
// is bare field=value, INFERRED from the FreshBooks docs (see spec section
// 5.1's STATE AS OF callout); this is the first business-scoped list
// endpoint this library implements against it.
//
// There is no Page/PerPage here: FreshBooks' Postman example response for
// this endpoint carries no "meta" pagination block, so there is nothing
// confirmed to paginate against yet. Add them back (and an All iterator)
// once a live call confirms the block exists.
type RetainerListOptions struct {
	Search Search
}

func (o *RetainerListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, 0, 0)
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
	return newPage(resp.Retainers, resp.Meta), nil
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

// RetainerCreateRequest is the payload for Create. Fee and ExcessRate are
// json.Number rather than float64: FreshBooks sends and accepts them as
// bare JSON numbers on the wire, but the package avoids binary floating
// point for money everywhere else (see Money), and a caller building a
// json.Number from a decimal string keeps that same guarantee here.
type RetainerCreateRequest struct {
	ClientUserID          string      `json:"client_user_id"`
	StartDate             string      `json:"start_date"`
	NextPeriodStartDate   string      `json:"next_period_start_date,omitempty"`
	Fee                   json.Number `json:"fee"`
	ExcessRate            json.Number `json:"excess_rate,omitempty"`
	BudgetedTime          int64       `json:"budgeted_time,omitempty"`
	Active                bool        `json:"active"`
	Frequency             string      `json:"frequency,omitempty"`
	NumberRecurring       int         `json:"number_recurring,omitempty"`
	IsInfinitelyRecurring bool        `json:"is_infinitely_recurring,omitempty"`
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

// RetainerUpdateRequest is the payload for Update. Active is a pointer, like
// every sibling update struct in this batch: Update is a partial write, and
// a caller who sets Fee without touching Active must not silently
// deactivate the retainer by sending an implicit false.
type RetainerUpdateRequest struct {
	ClientUserID          string      `json:"client_user_id,omitempty"`
	StartDate             string      `json:"start_date,omitempty"`
	NextPeriodStartDate   string      `json:"next_period_start_date,omitempty"`
	Fee                   json.Number `json:"fee,omitempty"`
	ExcessRate            json.Number `json:"excess_rate,omitempty"`
	BudgetedTime          int64       `json:"budgeted_time,omitempty"`
	Active                *bool       `json:"active,omitempty"`
	Frequency             string      `json:"frequency,omitempty"`
	NumberRecurring       int         `json:"number_recurring,omitempty"`
	IsInfinitelyRecurring bool        `json:"is_infinitely_recurring,omitempty"`
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
