package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// BillVendorsService is the vendors resource that vendor bills bill against.
// Bills/Vendors are a Beta endpoint in the Postman collection ("Vendors
// (Beta)"); facts here are INFERRED from the collection's examples and the
// FreshBooks docs page, not live-verified.
type BillVendorsService struct{ client *Client }

// BillVendor is a vendor that bills can be raised against.
type BillVendor struct {
	// VendorID is the vendor's identifier.
	VendorID int64 `json:"vendorid"`
	// VendorName is the vendor's display name.
	VendorName string `json:"vendor_name"`
	// AccountNumber is the account number FreshBooks uses with this vendor.
	AccountNumber string `json:"account_number,omitempty"`
	// PrimaryContactFirstName and PrimaryContactLastName name the vendor's
	// primary contact.
	PrimaryContactFirstName string `json:"primary_contact_first_name,omitempty"`
	PrimaryContactLastName  string `json:"primary_contact_last_name,omitempty"`
	// PrimaryContactEmail is the primary contact's email address.
	PrimaryContactEmail string `json:"primary_contact_email,omitempty"`
	// Street, Street2, City, Province, PostalCode, and Country are the
	// vendor's address.
	Street     string `json:"street,omitempty"`
	Street2    string `json:"street2,omitempty"`
	City       string `json:"city,omitempty"`
	Province   string `json:"province,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country,omitempty"`
	// Phone is the vendor's phone number.
	Phone string `json:"phone,omitempty"`
	// Website is the vendor's website.
	Website string `json:"website,omitempty"`
	// CurrencyCode is the vendor's ISO 4217 currency.
	CurrencyCode string `json:"currency_code,omitempty"`
	// Language is the vendor's preferred language code.
	Language string `json:"language,omitempty"`
	// Is1099 reports whether this vendor requires 1099 tracking (US tax).
	Is1099 bool `json:"is_1099,omitempty"`
	// TaxDefaults are the tax names applied to bills from this vendor by
	// default.
	TaxDefaults []string `json:"tax_defaults,omitempty"`
	// CreatedAt and UpdatedAt are account-local timestamps.
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	// VisState is the vendor's visibility state.
	VisState VisState `json:"vis_state"`
}

type billVendorEnvelope struct {
	BillVendor BillVendor `json:"bill_vendor"`
}

type billVendorListEnvelope struct {
	BillVendors []BillVendor `json:"bill_vendors"`
	PageMeta
}

// BillVendorRequest is the payload for Create and Update. VendorName is
// required by the API on creation.
type BillVendorRequest struct {
	VendorName              string   `json:"vendor_name,omitempty"`
	PrimaryContactFirstName string   `json:"primary_contact_first_name,omitempty"`
	PrimaryContactLastName  string   `json:"primary_contact_last_name,omitempty"`
	PrimaryContactEmail     string   `json:"primary_contact_email,omitempty"`
	Street                  string   `json:"street,omitempty"`
	Street2                 string   `json:"street2,omitempty"`
	City                    string   `json:"city,omitempty"`
	Province                string   `json:"province,omitempty"`
	PostalCode              string   `json:"postal_code,omitempty"`
	Country                 string   `json:"country,omitempty"`
	AccountNumber           string   `json:"account_number,omitempty"`
	Phone                   string   `json:"phone,omitempty"`
	Website                 string   `json:"website,omitempty"`
	CurrencyCode            string   `json:"currency_code,omitempty"`
	Language                string   `json:"language,omitempty"`
	Is1099                  bool     `json:"is_1099,omitempty"`
	TaxDefaults             []string `json:"tax_defaults,omitempty"`
}

// BillVendorListOptions filters and paginates List.
type BillVendorListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *BillVendorListOptions) requestOptions() []RequestOption {
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

func billVendorsPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/bill_vendors/bill_vendors", acct)
}

func billVendorPath(acct AccountID, id int64) string {
	return fmt.Sprintf("/accounting/account/%s/bill_vendors/bill_vendors/%d", acct, id)
}

// List returns one page of vendors.
//
// inventory: Expenses/Vendors (Beta)/Get Vendors
func (s *BillVendorsService) List(ctx context.Context, acct AccountID, opts *BillVendorListOptions) (*Page[BillVendor], error) {
	var env billVendorListEnvelope
	if err := s.client.do(ctx, http.MethodGet, billVendorsPath(acct), FamilyAccounting, nil, &env, opts.requestOptions()...); err != nil {
		return nil, err
	}
	return &Page[BillVendor]{Items: env.BillVendors, Page: env.Page, Pages: env.Pages, PerPage: env.PerPage, Total: env.Total}, nil
}

// All walks every page of vendors, auto-paginating.
func (s *BillVendorsService) All(ctx context.Context, acct AccountID, opts *BillVendorListOptions) iter.Seq2[BillVendor, error] {
	perPage := 100
	var search Search
	if opts != nil {
		if opts.PerPage > 0 {
			perPage = opts.PerPage
		}
		search = opts.Search
	}
	return All(ctx, func(ctx context.Context, page int) (*Page[BillVendor], error) {
		return s.List(ctx, acct, &BillVendorListOptions{Search: search, Page: page, PerPage: perPage})
	})
}

// Create adds a new vendor.
//
// inventory: Expenses/Vendors (Beta)/Add Vendor
func (s *BillVendorsService) Create(ctx context.Context, acct AccountID, req *BillVendorRequest) (*BillVendor, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: BillVendors.Create needs a request")
	}
	body := struct {
		BillVendor *BillVendorRequest `json:"bill_vendor"`
	}{req}
	var env billVendorEnvelope
	if err := s.client.do(ctx, http.MethodPost, billVendorsPath(acct), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.BillVendor, nil
}

// Update changes an existing vendor's details.
//
// inventory: Expenses/Vendors (Beta)/Edit Vendor Details
func (s *BillVendorsService) Update(ctx context.Context, acct AccountID, id int64, req *BillVendorRequest) (*BillVendor, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: BillVendors.Update needs a request")
	}
	body := struct {
		BillVendor *BillVendorRequest `json:"bill_vendor"`
	}{req}
	var env billVendorEnvelope
	if err := s.client.do(ctx, http.MethodPut, billVendorPath(acct, id), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.BillVendor, nil
}

// Delete soft-deletes a vendor by setting vis_state to 1. FreshBooks models
// this as a PUT, not a real HTTP DELETE.
//
// inventory: Expenses/Vendors (Beta)/Delete Vendor
func (s *BillVendorsService) Delete(ctx context.Context, acct AccountID, id int64) error {
	body := map[string]any{"bill_vendor": map[string]any{"vis_state": VisStateDeleted}}
	return s.client.do(ctx, http.MethodPut, billVendorPath(acct, id), FamilyAccounting, body, nil)
}
