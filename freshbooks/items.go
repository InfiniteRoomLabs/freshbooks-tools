package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// ItemsService (declared in services.go) is the accounting items and
// services catalogue: the products invoices bill line items against.
// FreshBooks' Postman collection lists every one of these requests twice,
// once under Invoices/Items and Services and once under Settings/Items and
// Services; both names map to the methods here.

// Item is a catalogue item or service.
type Item struct {
	ID                 int64  `json:"id"`
	ItemID             int64  `json:"itemid"`
	AccountingSystemID string `json:"accounting_systemid,omitempty"`
	Name               string `json:"name"`
	Description        string `json:"description,omitempty"`
	SKU                string `json:"sku,omitempty"`
	// Qty and Inventory are decimal strings, matching how FreshBooks sends
	// them.
	Qty       string `json:"qty,omitempty"`
	Inventory string `json:"inventory,omitempty"`
	UnitCost  Money  `json:"unit_cost"`
	// Tax1 and Tax2 are the ids of the item's default taxes (FreshBooks'
	// docs: "id of default tax for the item"), not rates -- QA caught this
	// modeled as a float64.
	Tax1     *int64   `json:"tax1,omitempty"`
	Tax2     *int64   `json:"tax2,omitempty"`
	Updated  DateTime `json:"updated,omitempty"`
	VisState VisState `json:"vis_state"`
}

type itemEnvelope struct {
	Item *Item `json:"item"`
}

type itemWriteEnvelope struct {
	Item any `json:"item"`
}

type itemListResponse struct {
	Items []Item `json:"items"`
	PageMeta
}

// ItemListOptions selects and paginates List. Set Search{"sku": "..."} to
// filter by SKU.
type ItemListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *ItemListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

// List returns one page of catalogue items. Filtering by SKU (the Postman
// collection's separate "List Items Filtered by SKU" request) is the same
// call with Search{"sku": "..."} set.
//
// inventory: Invoices/Items and Services/List Items
// inventory: Invoices/Items and Services/List Items Filtered by SKU
// inventory: Settings/Items and Services/List Items
// inventory: Settings/Items and Services/List Items Filtered by SKU
func (s *ItemsService) List(ctx context.Context, acct AccountID, opts *ItemListOptions, extra ...RequestOption) (*Page[Item], error) {
	path, err := itemsPath(acct)
	if err != nil {
		return nil, err
	}
	var resp itemListResponse
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.Items, resp.PageMeta), nil
}

// All iterates every catalogue item across every page.
func (s *ItemsService) All(ctx context.Context, acct AccountID, opts *ItemListOptions, extra ...RequestOption) iter.Seq2[Item, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Item], error) {
		o := ItemListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		return s.List(ctx, acct, &o, extra...)
	})
}

// Get fetches one catalogue item by id.
//
// inventory: Invoices/Items and Services/Single Item
// inventory: Settings/Items and Services/Single Item
func (s *ItemsService) Get(ctx context.Context, acct AccountID, id int64) (*Item, error) {
	path, err := itemPath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp itemEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Item, nil
}

// ItemCreateRequest is the payload for Create.
type ItemCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SKU         string `json:"sku,omitempty"`
	Qty         string `json:"qty,omitempty"`
	Inventory   string `json:"inventory,omitempty"`
	UnitCost    Money  `json:"unit_cost,omitzero"`
}

// Create adds a new catalogue item.
//
// inventory: Invoices/Items and Services/Create Item
// inventory: Settings/Items and Services/Create Item
func (s *ItemsService) Create(ctx context.Context, acct AccountID, req *ItemCreateRequest) (*Item, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Items.Create needs a request")
	}
	path, err := itemsPath(acct)
	if err != nil {
		return nil, err
	}
	var resp itemEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, itemWriteEnvelope{Item: req}, &resp); err != nil {
		return nil, err
	}
	return resp.Item, nil
}

// ItemUpdateRequest is the payload for Update. Every field is optional; only
// set ones are sent.
type ItemUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	SKU         *string `json:"sku,omitempty"`
	Qty         *string `json:"qty,omitempty"`
	Inventory   *string `json:"inventory,omitempty"`
	UnitCost    *Money  `json:"unit_cost,omitempty"`
}

// Update changes fields on an existing catalogue item.
//
// inventory: Invoices/Items and Services/Update Item
// inventory: Settings/Items and Services/Update Item
func (s *ItemsService) Update(ctx context.Context, acct AccountID, id int64, req *ItemUpdateRequest) (*Item, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Items.Update needs a request")
	}
	path, err := itemPath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp itemEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, itemWriteEnvelope{Item: req}, &resp); err != nil {
		return nil, err
	}
	return resp.Item, nil
}

// Delete soft-deletes a catalogue item by setting its visibility state.
//
// inventory: Invoices/Items and Services/Delete Item
// inventory: Settings/Items and Services/Delete Item
func (s *ItemsService) Delete(ctx context.Context, acct AccountID, id int64) error {
	path, err := itemPath(acct, id)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "item")
}

func itemsPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/items/items", acct), nil
}

func itemPath(acct AccountID, id int64) (string, error) {
	base, err := itemsPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}
