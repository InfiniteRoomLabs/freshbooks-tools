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

// BillVendorTaxDefault is one default tax FreshBooks applies to bills raised
// against a vendor.
type BillVendorTaxDefault struct {
	TaxID  int64  `json:"taxid,omitempty"`
	Name   string `json:"name,omitempty"`
	Amount string `json:"amount,omitempty"`
}

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
	// TaxDefaults are the default taxes applied to bills from this vendor.
	TaxDefaults []BillVendorTaxDefault `json:"tax_defaults,omitempty"`
	// OutstandingBalance is the total unpaid balance owed to this vendor.
	OutstandingBalance *Money `json:"outstanding_balance,omitempty"`
	// CreatedAt and UpdatedAt are account-local timestamps.
	CreatedAt DateTime `json:"created_at,omitempty"`
	UpdatedAt DateTime `json:"updated_at,omitempty"`
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
	VendorName              string `json:"vendor_name,omitempty"`
	PrimaryContactFirstName string `json:"primary_contact_first_name,omitempty"`
	PrimaryContactLastName  string `json:"primary_contact_last_name,omitempty"`
	PrimaryContactEmail     string `json:"primary_contact_email,omitempty"`
	Street                  string `json:"street,omitempty"`
	Street2                 string `json:"street2,omitempty"`
	City                    string `json:"city,omitempty"`
	Province                string `json:"province,omitempty"`
	PostalCode              string `json:"postal_code,omitempty"`
	Country                 string `json:"country,omitempty"`
	AccountNumber           string `json:"account_number,omitempty"`
	Phone                   string `json:"phone,omitempty"`
	Website                 string `json:"website,omitempty"`
	CurrencyCode            string `json:"currency_code,omitempty"`
	Language                string `json:"language,omitempty"`
	// Is1099 is a pointer so a caller can explicitly send false to turn 1099
	// tracking off, not just omit the field to leave it unset.
	Is1099      *bool    `json:"is_1099,omitempty"`
	TaxDefaults []string `json:"tax_defaults,omitempty"`
}

// BillVendorListOptions filters and paginates List.
type BillVendorListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *BillVendorListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

func billVendorsPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/bill_vendors/bill_vendors", acct), nil
}

func billVendorPath(acct AccountID, id int64) (string, error) {
	base, err := billVendorsPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}

// List returns one page of vendors.
//
// inventory: Expenses/Vendors (Beta)/Get Vendors
func (s *BillVendorsService) List(ctx context.Context, acct AccountID, opts *BillVendorListOptions, extra ...RequestOption) (*Page[BillVendor], error) {
	path, err := billVendorsPath(acct)
	if err != nil {
		return nil, err
	}
	var env billVendorListEnvelope
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(env.BillVendors, env.PageMeta), nil
}

// All walks every page of vendors, auto-paginating.
func (s *BillVendorsService) All(ctx context.Context, acct AccountID, opts *BillVendorListOptions, extra ...RequestOption) iter.Seq2[BillVendor, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[BillVendor], error) {
		o := BillVendorListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		o.PerPage = pageSize(o.PerPage)
		return s.List(ctx, acct, &o, extra...)
	})
}

// Create adds a new vendor.
//
// inventory: Expenses/Vendors (Beta)/Add Vendor
func (s *BillVendorsService) Create(ctx context.Context, acct AccountID, req *BillVendorRequest) (*BillVendor, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: BillVendors.Create needs a request")
	}
	path, err := billVendorsPath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		BillVendor *BillVendorRequest `json:"bill_vendor"`
	}{req}
	var env billVendorEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
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
	path, err := billVendorPath(acct, id)
	if err != nil {
		return nil, err
	}
	body := struct {
		BillVendor *BillVendorRequest `json:"bill_vendor"`
	}{req}
	var env billVendorEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.BillVendor, nil
}

// Delete soft-deletes a vendor by setting vis_state to 1. FreshBooks models
// this as a PUT, not a real HTTP DELETE.
//
// inventory: Expenses/Vendors (Beta)/Delete Vendor
func (s *BillVendorsService) Delete(ctx context.Context, acct AccountID, id int64) error {
	path, err := billVendorPath(acct, id)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "bill_vendor")
}
