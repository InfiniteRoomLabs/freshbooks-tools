package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// Service is a project-services-catalogue entry: something a business
// offers that time entries and projects associate with. This is the
// business-family "service" record read through the comments API, distinct
// from the accounting-family billable-item record BillableItem models --
// the Postman collection's "Settings/Items and Services" folder mixes both
// under one name; see BillableItem's doc comment for the split.
type Service struct {
	ID             int64    `json:"id"`
	BusinessID     int64    `json:"business_id"`
	Name           string   `json:"name"`
	Billable       bool     `json:"billable"`
	ProjectDefault bool     `json:"project_default"`
	VisState       VisState `json:"vis_state"`
}

type serviceResponse struct {
	Service Service `json:"service"`
}

// Get returns one service by ID.
//
// inventory: Settings/Items and Services/Get a Single Service
func (s *ServicesService) Get(ctx context.Context, businessID BusinessID, serviceID int64) (*Service, error) {
	var resp serviceResponse
	path := fmt.Sprintf("/comments/business/%s/service/%d", businessID, serviceID)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Service, nil
}

type servicesListResponse struct {
	Services []Service `json:"services"`
	Meta     PageMeta  `json:"meta"`
}

// ServiceListOptions filters and paginates List.
type ServiceListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *ServiceListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

// List returns one page of businessID's services.
//
// inventory: Settings/Items and Services/List Services
func (s *ServicesService) List(ctx context.Context, businessID BusinessID, opts *ServiceListOptions, extra ...RequestOption) (*Page[Service], error) {
	var resp servicesListResponse
	path := "/comments/business/" + businessID.String() + "/services"
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.Services, resp.Meta), nil
}

// BillableItem is the accounting-family billable-item record: the same
// underlying resource Projects/Tasks addresses under
// /accounting/account/{account_id}/projects/tasks (that folder's response
// shares this shape's billable, tax1, tax2, and unit_cost fields). Create
// posts here instead of to the lighter business-family Service record above
// -- a genuine split in the Postman collection between two "service"
// concepts, not a duplicate.
type BillableItem struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Billable    bool     `json:"billable"`
	Tax1        int      `json:"tax1"`
	Tax2        int      `json:"tax2"`
	UnitCost    Money    `json:"unit_cost"`
	Updated     DateTime `json:"updated"`
	VisState    VisState `json:"vis_state"`
}

type billableItemResponse struct {
	BillableItem BillableItem `json:"billable_item"`
}

// BillableItemCreateRequest is the payload for Create.
type BillableItemCreateRequest struct {
	Name        string `json:"name"`
	Billable    bool   `json:"billable"`
	Description string `json:"description,omitempty"`
	Tax1        *int   `json:"tax1,omitempty"`
	Tax2        *int   `json:"tax2,omitempty"`
	UnitCost    *Money `json:"unit_cost,omitempty"`
}

func billableItemsPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/billable_items/billable_items", acct), nil
}

func billableItemPath(acct AccountID, id int64) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/billable_items/%d", acct, id), nil
}

// Create adds a billable item. The Postman collection carries no response
// example for this endpoint; BillableItem's shape is INFERRED from the
// sibling Single Service response and the request body's own fields, not
// observed live.
//
// inventory: Settings/Items and Services/Create Service
func (s *ServicesService) Create(ctx context.Context, accountID AccountID, req *BillableItemCreateRequest) (*BillableItem, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Create needs a request")
	}
	path, err := billableItemsPath(accountID)
	if err != nil {
		return nil, err
	}
	var resp billableItemResponse
	body := map[string]*BillableItemCreateRequest{"billable_item": req}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.BillableItem, nil
}

// GetBillableItem returns one billable item by ID -- the accounting-family
// counterpart to Get. Its response shape is INFERRED (see Create); the
// Postman collection carries no example for this endpoint either.
//
// inventory: Settings/Items and Services/Single Service
func (s *ServicesService) GetBillableItem(ctx context.Context, accountID AccountID, id int64) (*BillableItem, error) {
	path, err := billableItemPath(accountID, id)
	if err != nil {
		return nil, err
	}
	var resp billableItemResponse
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.BillableItem, nil
}
