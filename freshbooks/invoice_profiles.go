package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// InvoiceProfilesService (declared in services.go) is the accounting
// invoice_profiles resource, FreshBooks' recurring-invoice templates.
// FreshBooks' UI calls these "recurring templates"; the API and this
// package call them profiles.

// LateFee describes an invoice profile's late-payment penalty.
type LateFee struct {
	Enabled          bool    `json:"enabled"`
	Days             int     `json:"days,omitempty"`
	Type             string  `json:"type,omitempty"` // "percent" or "flat"
	Value            float64 `json:"value,omitempty"`
	Repeat           bool    `json:"repeat,omitempty"`
	CompoundedTaxes  bool    `json:"compounded_taxes,omitempty"`
	FirstTaxName     string  `json:"first_tax_name,omitempty"`
	FirstTaxPercent  float64 `json:"first_tax_percent,omitempty"`
	SecondTaxName    string  `json:"second_tax_name,omitempty"`
	SecondTaxPercent float64 `json:"second_tax_percent,omitempty"`
}

// ProjectFormat controls how an invoice profile bills unbilled time entries.
type ProjectFormat struct {
	GroupBy         string `json:"group_by,omitempty"`
	Method          string `json:"method,omitempty"`
	ShowTaskName    bool   `json:"show_tname,omitempty"`
	ShowProjectName bool   `json:"show_pname,omitempty"`
	ShowStaff       bool   `json:"show_staff,omitempty"`
	ShowDate        bool   `json:"show_date,omitempty"`
	ShowNotes       bool   `json:"show_notes,omitempty"`
	ShowTotalHours  bool   `json:"show_total_hours,omitempty"`
}

// InvoiceProfile is a FreshBooks recurring-invoice template.
type InvoiceProfile struct {
	ID                 int64    `json:"id"`
	ProfileID          int64    `json:"profileid"`
	AccountingSystemID string   `json:"accounting_systemid,omitempty"`
	CustomerID         int64    `json:"customerid"`
	OwnerID            int64    `json:"ownerid,omitempty"`
	VisState           VisState `json:"vis_state"`
	Disable            bool     `json:"disable"`

	// Frequency is FreshBooks' shorthand recurrence code, e.g. "m" monthly,
	// "2w" every two weeks.
	Frequency           string `json:"frequency,omitempty"`
	NumberRecurring     int    `json:"numberRecurring"`
	OccurrencesToDate   int    `json:"occurrences_to_date,omitempty"`
	IncludeUnbilledTime bool   `json:"include_unbilled_time"`
	RequireAutoBill     bool   `json:"require_auto_bill"`
	AutoBill            bool   `json:"auto_bill"`
	SendEmail           bool   `json:"send_email"`
	SendGmail           bool   `json:"send_gmail"`
	DueOffsetDays       int    `json:"due_offset_days,omitempty"`
	RetainerID          *int64 `json:"retainer_id,omitempty"`
	// ExtArchive is FreshBooks' deprecated 0/1 archived flag, nullable in
	// the captured example.
	ExtArchive *int64 `json:"ext_archive,omitempty"`
	// TotalAccruedRevenue is a documented field with no captured example
	// showing a non-null value or a docs type; modeled as a nullable Money
	// like this struct's Amount/DiscountTotal rather than confirmed.
	TotalAccruedRevenue *Money `json:"total_accrued_revenue,omitempty"`

	CreateDate   Date     `json:"create_date"`
	Updated      DateTime `json:"updated,omitempty"`
	CurrencyCode string   `json:"currency_code,omitempty"`
	Language     string   `json:"language,omitempty"`

	Amount        Money  `json:"amount"`
	DiscountTotal Money  `json:"discount_total"`
	DiscountValue string `json:"discount_value,omitempty"`

	Organization   string  `json:"organization,omitempty"`
	FName          string  `json:"fname,omitempty"`
	LName          string  `json:"lname,omitempty"`
	Address        string  `json:"address,omitempty"`
	Street         string  `json:"street,omitempty"`
	Street2        string  `json:"street2,omitempty"`
	City           string  `json:"city,omitempty"`
	Province       string  `json:"province,omitempty"`
	Code           string  `json:"code,omitempty"`
	Country        string  `json:"country,omitempty"`
	VatName        string  `json:"vat_name,omitempty"`
	VatNumber      string  `json:"vat_number,omitempty"`
	Notes          string  `json:"notes,omitempty"`
	Terms          *string `json:"terms,omitempty"`
	PONumber       *string `json:"po_number,omitempty"`
	PaymentDetails string  `json:"payment_details,omitempty"`
	Description    string  `json:"description,omitempty"`
	BillGateway    *string `json:"bill_gateway,omitempty"`

	// Lines is populated on write and when Include("lines") is used.
	Lines []InvoiceLine `json:"lines,omitempty"`
	// LateFee and ProjectFormat are populated when Include("late_fee") /
	// Include("project_format") is used.
	LateFee       *LateFee       `json:"late_fee,omitempty"`
	ProjectFormat *ProjectFormat `json:"project_format,omitempty"`
}

type invoiceProfileEnvelope struct {
	InvoiceProfile *InvoiceProfile `json:"invoice_profile"`
}

type invoiceProfileWriteEnvelope struct {
	InvoiceProfile any `json:"invoice_profile"`
}

type invoiceProfileListResponse struct {
	InvoiceProfiles []InvoiceProfile `json:"invoice_profiles"`
	PageMeta
}

// InvoiceProfileListOptions selects and paginates List.
type InvoiceProfileListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *InvoiceProfileListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

// List returns one page of invoice profiles.
//
// inventory: Invoices/Invoice Recurring Template/List Invoice Profiles
func (s *InvoiceProfilesService) List(ctx context.Context, acct AccountID, opts *InvoiceProfileListOptions, extra ...RequestOption) (*Page[InvoiceProfile], error) {
	path, err := invoiceProfilesPath(acct)
	if err != nil {
		return nil, err
	}
	var resp invoiceProfileListResponse
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.InvoiceProfiles, resp.PageMeta), nil
}

// All iterates every invoice profile across every page.
func (s *InvoiceProfilesService) All(ctx context.Context, acct AccountID, opts *InvoiceProfileListOptions, extra ...RequestOption) iter.Seq2[InvoiceProfile, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[InvoiceProfile], error) {
		o := InvoiceProfileListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		return s.List(ctx, acct, &o, extra...)
	})
}

// Get fetches one invoice profile by id.
//
// inventory: Invoices/Invoice Recurring Template/Get Single Invoice Profile
func (s *InvoiceProfilesService) Get(ctx context.Context, acct AccountID, id int64, opts ...RequestOption) (*InvoiceProfile, error) {
	path, err := invoiceProfilePath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp invoiceProfileEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.InvoiceProfile, nil
}

// InvoiceProfileCreateRequest is the payload for Create.
type InvoiceProfileCreateRequest struct {
	CustomerID          int64          `json:"customerid"`
	CreateDate          Date           `json:"create_date,omitzero"`
	Frequency           string         `json:"frequency"`
	NumberRecurring     int            `json:"numberRecurring"`
	Lines               []InvoiceLine  `json:"lines,omitempty"`
	Disable             *bool          `json:"disable,omitempty"`
	DueOffsetDays       int            `json:"due_offset_days,omitempty"`
	SendEmail           *bool          `json:"send_email,omitempty"`
	IncludeUnbilledTime *bool          `json:"include_unbilled_time,omitempty"`
	ProjectFormat       *ProjectFormat `json:"project_format,omitempty"`
	RetainerID          *int64         `json:"retainer_id,omitempty"`
	RequireAutoBill     *bool          `json:"require_auto_bill,omitempty"`
	CurrencyCode        string         `json:"currency_code,omitempty"`
	DiscountValue       string         `json:"discount_value,omitempty"`
	Template            string         `json:"template,omitempty"`
	Terms               string         `json:"terms,omitempty"`
	AllowedGatewayIDs   []int64        `json:"allowed_gatewayids,omitempty"`
	LateFee             *LateFee       `json:"late_fee,omitempty"`
}

// Create makes a new recurring invoice template. A plain template and one
// that pre-fills unbilled time entries (ProjectFormat, IncludeUnbilledTime)
// POST the same endpoint with the same body shape, so they share this one
// method.
//
// inventory: Invoices/Invoice Recurring Template/Create Single Invoice Profile
// inventory: Invoices/Invoice Recurring Template/Create Single Invoice Profile w/ Time Entry Holder
func (s *InvoiceProfilesService) Create(ctx context.Context, acct AccountID, req *InvoiceProfileCreateRequest, opts ...RequestOption) (*InvoiceProfile, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: InvoiceProfiles.Create needs a request")
	}
	path, err := invoiceProfilesPath(acct)
	if err != nil {
		return nil, err
	}
	var resp invoiceProfileEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, invoiceProfileWriteEnvelope{InvoiceProfile: req}, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.InvoiceProfile, nil
}

// InvoiceProfileUpdateRequest is the payload for Update. Every field is
// optional; only set ones are sent.
type InvoiceProfileUpdateRequest struct {
	Frequency       *string `json:"frequency,omitempty"`
	NumberRecurring *int    `json:"numberRecurring,omitempty"`
	Disable         *bool   `json:"disable,omitempty"`
	SendEmail       *bool   `json:"send_email,omitempty"`
}

// Update changes fields on an existing invoice profile.
//
// inventory: Invoices/Invoice Recurring Template/Update Invoice Profile
func (s *InvoiceProfilesService) Update(ctx context.Context, acct AccountID, id int64, req *InvoiceProfileUpdateRequest) (*InvoiceProfile, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: InvoiceProfiles.Update needs a request")
	}
	path, err := invoiceProfilePath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp invoiceProfileEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, invoiceProfileWriteEnvelope{InvoiceProfile: req}, &resp); err != nil {
		return nil, err
	}
	return resp.InvoiceProfile, nil
}

// Delete soft-deletes an invoice profile by setting its visibility state.
//
// inventory: Invoices/Invoice Recurring Template/Delete  Invoice Profile
func (s *InvoiceProfilesService) Delete(ctx context.Context, acct AccountID, id int64) error {
	path, err := invoiceProfilePath(acct, id)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "invoice_profile")
}

// EnablePaymentOptions turns on FreshBooks Payments for a recurring invoice
// profile: which gateway to use, and which payment methods it accepts. See
// paymentOptionsBody's doc comment (invoices.go) for the entity_type/
// entity_id fields this sends and why.
//
// inventory: Invoices/Invoice Recurring Template/Enable Payment Options On Invoice Profile
func (s *InvoiceProfilesService) EnablePaymentOptions(ctx context.Context, acct AccountID, id int64, req *PaymentOptionsRequest) error {
	if req == nil {
		return fmt.Errorf("freshbooks: InvoiceProfiles.EnablePaymentOptions needs a request")
	}
	if err := pathSegment(string(acct)); err != nil {
		return err
	}
	path := fmt.Sprintf("/payments/account/%s/invoice_profile/%d/payment_options", acct, id)
	body := paymentOptionsBody{PaymentOptionsRequest: *req, EntityType: "invoice_profile", EntityID: id}
	return s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, nil)
}

func invoiceProfilesPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/invoice_profiles/invoice_profiles", acct), nil
}

func invoiceProfilePath(acct AccountID, id int64) (string, error) {
	base, err := invoiceProfilesPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}
