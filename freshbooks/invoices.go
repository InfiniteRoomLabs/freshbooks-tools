package freshbooks

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"net/http"
)

// InvoicesService (declared in services.go) is the accounting invoices
// resource: creating, listing, updating, sending, and downloading client
// invoices.

// InvoiceLine is one line item on an invoice or invoice profile. Type 0 is a
// standard line, type 1 bills an expense (ExpenseID set), type 5 is a
// recurring-template placeholder line.
type InvoiceLine struct {
	// LineID identifies an existing line; omit when adding a new one.
	LineID int64 `json:"lineid,omitempty"`
	// Type selects the line kind: 0 standard, 1 expense, 5 template.
	Type int `json:"type"`
	// Description is the line's free-text detail.
	Description string `json:"description,omitempty"`
	// Name is the line's short title.
	Name string `json:"name,omitempty"`
	// Qty is the quantity billed.
	Qty float64 `json:"qty,omitempty"`
	// UnitCost is the price per unit.
	UnitCost Money `json:"unit_cost,omitzero"`
	// ExpenseID bills a specific expense when Type is 1.
	ExpenseID int64 `json:"expenseid,omitempty"`
	// TaxName1, TaxAmount1, TaxName2, TaxAmount2 are the line's up-to-two
	// applied taxes.
	TaxName1   string  `json:"taxName1,omitempty"`
	TaxAmount1 float64 `json:"taxAmount1,omitempty"`
	TaxName2   string  `json:"taxName2,omitempty"`
	TaxAmount2 float64 `json:"taxAmount2,omitempty"`
	// Amount is the line's computed total; the server fills it, callers
	// never send it.
	Amount Money `json:"amount,omitzero"`
}

// InvoicePresentation controls an invoice's branded appearance: logo,
// banner, theme, and heading overrides.
type InvoicePresentation struct {
	// ImageLogoSrc and ImageBannerSrc are upload-service paths, produced by
	// the Uploader resource.
	ImageLogoSrc   string `json:"image_logo_src,omitempty"`
	ImageBannerSrc string `json:"image_banner_src,omitempty"`
	// ImageBannerPositionY offsets the banner image vertically.
	ImageBannerPositionY int `json:"image_banner_position_y,omitempty"`
	// ThemePrimaryColor is a hex color, e.g. "#663399".
	ThemePrimaryColor string `json:"theme_primary_color,omitempty"`
	// ThemeLayout and ThemeFontName select the presentation template.
	ThemeLayout   string `json:"theme_layout,omitempty"`
	ThemeFontName string `json:"theme_font_name,omitempty"`
	// DateFormat controls how dates render, e.g. "mm/dd/yyyy".
	DateFormat string `json:"date_format,omitempty"`
	// The remaining fields override the column and section headings; a nil
	// pointer leaves FreshBooks' default.
	DescriptionHeading    *string `json:"description_heading,omitempty"`
	ItemHeading           *string `json:"item_heading,omitempty"`
	QuantityHeading       *string `json:"quantity_heading,omitempty"`
	RateHeading           *string `json:"rate_heading,omitempty"`
	HoursHeading          *string `json:"hours_heading,omitempty"`
	TaskHeading           *string `json:"task_heading,omitempty"`
	UnitCostHeading       *string `json:"unit_cost_heading,omitempty"`
	TimeEntryNotesHeading *string `json:"time_entry_notes_heading,omitempty"`
	Label                 *string `json:"label,omitempty"`
}

// AllowedGateway is one payment gateway FreshBooks Payments has enabled for
// an invoice, checkout link, or invoice profile.
type AllowedGateway struct {
	ID           int64  `json:"id"`
	AllowedID    int64  `json:"allowedid"`
	GatewayID    int64  `json:"gatewayid"`
	GatewayName  string `json:"gateway_name"`
	ConnectionID string `json:"connectionid"`
}

// Invoice is a FreshBooks invoice. Read-only fields (ID, InvoiceID, Amount,
// Paid, Outstanding, timestamps, ...) are always populated by the server;
// write requests use InvoiceCreateRequest / InvoiceUpdateRequest instead of
// this type directly.
type Invoice struct {
	// ID and InvoiceID are the same numeric identifier under two names; the
	// API returns both.
	ID        int64 `json:"id"`
	InvoiceID int64 `json:"invoiceid"`
	// AccountID and AccountingSystemID both identify the owning account.
	AccountID          string `json:"accountid,omitempty"`
	AccountingSystemID string `json:"accounting_systemid,omitempty"`
	// InvoiceNumber is the human-facing invoice number, e.g. "0000013".
	InvoiceNumber string `json:"invoice_number,omitempty"`
	// CustomerID is the billed client's id.
	CustomerID int64 `json:"customerid"`
	OwnerID    int64 `json:"ownerid,omitempty"`
	// Status is FreshBooks' numeric invoice status; DisplayStatus and
	// V3Status are its human-readable derivatives ("draft", "sent",
	// "overdue", ...).
	Status         int     `json:"status"`
	DisplayStatus  string  `json:"display_status,omitempty"`
	V3Status       string  `json:"v3_status,omitempty"`
	PaymentStatus  string  `json:"payment_status,omitempty"`
	AutobillStatus *string `json:"autobill_status,omitempty"`
	DisputeStatus  *string `json:"dispute_status,omitempty"`
	// DepositStatus is a status string too, but FreshBooks always sends it
	// ("none" when no deposit applies) rather than omitting it.
	DepositStatus     string  `json:"deposit_status,omitempty"`
	DepositPercentage *string `json:"deposit_percentage,omitempty"`
	LastOrderStatus   *string `json:"last_order_status,omitempty"`
	// VisState is the visibility state; VisStateDeleted marks a
	// soft-deleted invoice.
	VisState VisState `json:"vis_state"`
	// UUID and Version are documented response fields with no captured
	// example in this batch's fixture source; Version looks like an
	// optimistic-concurrency token (docs example:
	// "2021-07-05 08:17:47.399872", not one of this package's three known
	// timestamp layouts), so it stays a plain string rather than a
	// DateTime this library cannot parse.
	UUID    string `json:"uuid,omitempty"`
	Version string `json:"version,omitempty"`
	// BasecampID, ExtArchive, and SentID are always-present integers in
	// every captured response (0/0/1 in the fixture); GMail is likewise
	// always present.
	BasecampID int64 `json:"basecampid,omitempty"`
	ExtArchive int64 `json:"ext_archive,omitempty"`
	SentID     int64 `json:"sentid,omitempty"`
	GMail      bool  `json:"gmail"`
	// NetPaidAmount is a documented response field with no captured
	// example; modeled as a nullable Money like this struct's other
	// *_amount fields (DepositAmount) rather than confirmed.
	NetPaidAmount *Money `json:"net_paid_amount,omitempty"`

	CreateDate      Date     `json:"create_date"`
	DueDate         Date     `json:"due_date,omitempty"`
	DatePaid        *Date    `json:"date_paid,omitempty"`
	FulfillmentDate *Date    `json:"fulfillment_date,omitempty"`
	GenerationDate  *Date    `json:"generation_date,omitempty"`
	CreatedAt       DateTime `json:"created_at,omitempty"`
	Updated         DateTime `json:"updated,omitempty"`
	DueOffsetDays   int      `json:"due_offset_days,omitempty"`

	CurrencyCode string  `json:"currency_code,omitempty"`
	Language     string  `json:"language,omitempty"`
	Notes        string  `json:"notes,omitempty"`
	Terms        string  `json:"terms,omitempty"`
	PONumber     *string `json:"po_number,omitempty"`
	Description  string  `json:"description,omitempty"`

	Amount              Money   `json:"amount"`
	Paid                Money   `json:"paid"`
	Outstanding         Money   `json:"outstanding"`
	DiscountTotal       Money   `json:"discount_total"`
	DiscountValue       string  `json:"discount_value,omitempty"`
	DiscountDescription *string `json:"discount_description,omitempty"`
	DepositAmount       *Money  `json:"deposit_amount,omitempty"`

	Organization        string `json:"organization,omitempty"`
	CurrentOrganization string `json:"current_organization,omitempty"`
	FName               string `json:"fname,omitempty"`
	LName               string `json:"lname,omitempty"`
	Address             string `json:"address,omitempty"`
	Street              string `json:"street,omitempty"`
	Street2             string `json:"street2,omitempty"`
	City                string `json:"city,omitempty"`
	Province            string `json:"province,omitempty"`
	Code                string `json:"code,omitempty"`
	Country             string `json:"country,omitempty"`
	VatName             string `json:"vat_name,omitempty"`
	VatNumber           string `json:"vat_number,omitempty"`

	Template        string  `json:"template,omitempty"`
	AutoBill        bool    `json:"auto_bill"`
	ShowAttachments bool    `json:"show_attachments"`
	ReturnURI       *string `json:"return_uri,omitempty"`
	PaymentDetails  string  `json:"payment_details,omitempty"`
	EstimateID      int64   `json:"estimateid,omitempty"`
	ParentID        int64   `json:"parent,omitempty"`

	// Lines is populated on write and, when Include("lines") is used, on
	// read.
	Lines []InvoiceLine `json:"lines,omitempty"`
	// Presentation is populated when Include("presentation") is used.
	Presentation *InvoicePresentation `json:"presentation,omitempty"`
	// AllowedGateways is populated when Include("allowed_gateways") is
	// used, or after Invoices.Update with AllowedGatewayIDs set, or
	// Invoices.EnablePaymentOptions.
	AllowedGateways []AllowedGateway `json:"allowed_gateways,omitempty"`
}

// invoiceEnvelope wraps a single invoice in the accounting family's key.
type invoiceEnvelope struct {
	Invoice *Invoice `json:"invoice"`
}

// invoiceListResponse is the accounting family's list-of-invoices result,
// with pagination fields alongside the plural key.
type invoiceListResponse struct {
	Invoices []Invoice `json:"invoices"`
	PageMeta
}

// InvoiceListOptions selects and paginates List.
type InvoiceListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *InvoiceListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

// List returns one page of invoices.
//
// inventory: Invoices/List Invoices
func (s *InvoicesService) List(ctx context.Context, acct AccountID, opts *InvoiceListOptions, extra ...RequestOption) (*Page[Invoice], error) {
	path, err := invoicesPath(acct)
	if err != nil {
		return nil, err
	}
	var resp invoiceListResponse
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.Invoices, resp.PageMeta), nil
}

// All iterates every invoice across every page.
func (s *InvoicesService) All(ctx context.Context, acct AccountID, opts *InvoiceListOptions, extra ...RequestOption) iter.Seq2[Invoice, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Invoice], error) {
		o := InvoiceListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		return s.List(ctx, acct, &o, extra...)
	})
}

// Get fetches one invoice by id. Pass Include("presentation") for the
// branded logo/theme fields, or Include("allowed_gateways") for the
// enabled-payment-gateway list.
//
// inventory: Invoices/Single Invoice
// inventory: Invoices/Single Invoice w/ Logo
func (s *InvoicesService) Get(ctx context.Context, acct AccountID, id int64, opts ...RequestOption) (*Invoice, error) {
	path, err := invoicePath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp invoiceEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.Invoice, nil
}

// InvoiceCreateRequest is the payload for Create.
type InvoiceCreateRequest struct {
	CustomerID   int64                `json:"customerid"`
	CreateDate   Date                 `json:"create_date,omitzero"`
	Lines        []InvoiceLine        `json:"lines,omitempty"`
	Presentation *InvoicePresentation `json:"presentation,omitempty"`
	Notes        string               `json:"notes,omitempty"`
	Terms        string               `json:"terms,omitempty"`
	PONumber     string               `json:"po_number,omitempty"`
	Language     string               `json:"language,omitempty"`
	CurrencyCode string               `json:"currency_code,omitempty"`
}

// Create makes a new invoice. FreshBooks' Postman collection names this
// operation three ways depending on what the example emphasizes (plain line
// items, a branded logo and theme, or an enabled payment gateway); all three
// POST the same endpoint with the same body shape, so they share this one
// method.
//
// inventory: Invoices/Create Invoice with Expense
// inventory: Invoices/Single Invoice w/ Line Items
// inventory: Invoices/Single Invoice w/ Logo and styles
// inventory: Invoices/Single Invoice w/ Payment Gateway
func (s *InvoicesService) Create(ctx context.Context, acct AccountID, req *InvoiceCreateRequest, opts ...RequestOption) (*Invoice, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Invoices.Create needs a request")
	}
	path, err := invoicesPath(acct)
	if err != nil {
		return nil, err
	}
	var resp invoiceEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, invoiceCreateEnvelope{Invoice: req}, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.Invoice, nil
}

// invoiceCreateEnvelope wraps a create/update request in the accounting
// family's "invoice" key.
type invoiceCreateEnvelope struct {
	Invoice any `json:"invoice"`
}

// InvoiceUpdateRequest is the payload for Update. Every field is optional;
// only set ones are sent. It also covers toggling which payment gateways an
// invoice accepts (AllowedGatewayIDs) since FreshBooks models that as an
// invoice field update, not a separate resource.
type InvoiceUpdateRequest struct {
	Status            *int                 `json:"status,omitempty"`
	Lines             []InvoiceLine        `json:"lines,omitempty"`
	Presentation      *InvoicePresentation `json:"presentation,omitempty"`
	Notes             *string              `json:"notes,omitempty"`
	Terms             *string              `json:"terms,omitempty"`
	AllowedGatewayIDs []int64              `json:"allowed_gatewayids,omitempty"`
}

// Update changes fields on an existing invoice. FreshBooks routes plain
// field edits, expense-line edits, and payment-gateway toggling through the
// same PUT, so they share this one method.
//
// inventory: Invoices/Update Invoice
// inventory: Invoices/Update Invoice w/ Expense
// inventory: Invoices/Toggle Online Payments on Invoice
func (s *InvoicesService) Update(ctx context.Context, acct AccountID, id int64, req *InvoiceUpdateRequest, opts ...RequestOption) (*Invoice, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Invoices.Update needs a request")
	}
	path, err := invoicePath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp invoiceEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, invoiceCreateEnvelope{Invoice: req}, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.Invoice, nil
}

// Delete soft-deletes an invoice by setting its visibility state. FreshBooks
// has no hard delete for invoices.
//
// inventory: Invoices/Delete  Invoice
func (s *InvoicesService) Delete(ctx context.Context, acct AccountID, id int64) error {
	path, err := invoicePath(acct, id)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "invoice")
}

// InvoiceSendRequest is the payload for Send.
type InvoiceSendRequest struct {
	// EmailRecipients overrides the client's default email; when empty
	// FreshBooks uses the client's own address.
	EmailRecipients []string `json:"email_recipients,omitempty"`
	// Subject and Body customize the notification email; leave both empty
	// for FreshBooks' default template.
	Subject string `json:"-"`
	Body    string `json:"-"`
}

type invoiceCustomizedEmail struct {
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

type invoiceSendBody struct {
	EmailRecipients        []string                `json:"email_recipients,omitempty"`
	InvoiceCustomizedEmail *invoiceCustomizedEmail `json:"invoice_customized_email,omitempty"`
	ActionEmail            bool                    `json:"action_email"`
}

// Send emails an invoice to its client, optionally overriding the recipient
// list and the email's subject and body.
//
// inventory: Invoices/Send Invoice by Email
func (s *InvoicesService) Send(ctx context.Context, acct AccountID, id int64, req *InvoiceSendRequest) error {
	path, err := invoicePath(acct, id)
	if err != nil {
		return err
	}
	body := invoiceSendBody{ActionEmail: true}
	if req != nil {
		body.EmailRecipients = req.EmailRecipients
		if req.Subject != "" || req.Body != "" {
			body.InvoiceCustomizedEmail = &invoiceCustomizedEmail{Subject: req.Subject, Body: req.Body}
		}
	}
	return s.client.do(ctx, http.MethodPut, path, FamilyAccounting, invoiceCreateEnvelope{Invoice: body}, nil)
}

// pdfMagic is the header every PDF file starts with; a body without it is
// not a PDF regardless of what its status code claimed.
var pdfMagic = []byte("%PDF-")

// PDF downloads an invoice as a rendered PDF. Unlike every other method in
// this package, the response is not JSON: it sends Accept: application/pdf
// (every other method sends application/json) and returns the raw bytes
// instead of decoding an envelope, but it shares the same
// resolve/authorize/retry/Retry-After path as every JSON method, and it
// rejects a 200 response whose body does not start with the PDF magic
// bytes -- an HTML interstitial or a login redirect page must not be
// handed to a caller as if it were a PDF.
//
// inventory: Invoices/Invoice Links/Downloads/Download Invoice PDF
func (s *InvoicesService) PDF(ctx context.Context, acct AccountID, id int64) ([]byte, error) {
	path, err := invoicePath(acct, id)
	if err != nil {
		return nil, err
	}
	raw, err := s.client.fetchRaw(ctx, http.MethodGet, path+"/pdf", FamilyAccounting, "application/pdf")
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(raw, pdfMagic) {
		return nil, fmt.Errorf("freshbooks: PDF response does not start with the PDF magic bytes")
	}
	return raw, nil
}

// InvoiceShareLink is a time-limited public URL for viewing or downloading
// an invoice without authentication. ShareLink itself is credential-
// equivalent: anyone holding it can view or download the invoice, so treat
// it like a secret in logs and default output (a CLI table/JSON dump or an
// MCP tool result that echoes it by default hands out the same access a
// bearer token would).
type InvoiceShareLink struct {
	ClientID     int64  `json:"clientid"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resourceid"`
	// ShareLink is the credential-equivalent URL; see the type's doc
	// comment.
	ShareLink   string `json:"share_link"`
	ShareMethod string `json:"share_method"`
}

type shareLinkEnvelope struct {
	ShareLink *InvoiceShareLink `json:"share_link"`
}

// ShareLink returns a public share link for a sent invoice, for either a
// web view or a direct PDF download; FreshBooks' Postman collection lists
// these as two named requests, but both call the same endpoint with the
// same share_method value. The returned link is credential-equivalent; see
// InvoiceShareLink's doc comment.
//
// inventory: Invoices/Invoice Links/Downloads/Share Link
// inventory: Invoices/Invoice Links/Downloads/Share PDF
func (s *InvoicesService) ShareLink(ctx context.Context, acct AccountID, id int64) (*InvoiceShareLink, error) {
	base, err := invoicePath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp shareLinkEnvelope
	// share_method is a fixed, bare query parameter (not a search[] filter),
	// so it is easiest to fold straight into the path FreshBooks' Postman
	// example always sends.
	path := base + "/share_link?share_method=share_link"
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return resp.ShareLink, nil
}

// PaymentOptionsRequest configures which payment methods FreshBooks Payments
// accepts for an invoice or invoice profile. Every FreshBooks example sends
// every boolean explicitly, including false -- this is a toggle endpoint,
// so an omitted field means "leave it as it is", not "off".
type PaymentOptionsRequest struct {
	GatewayName            string `json:"gateway_name,omitempty"`
	HasCreditCard          bool   `json:"has_credit_card"`
	HasACHTransfer         bool   `json:"has_ach_transfer"`
	HasBACSDebit           bool   `json:"has_bacs_debit"`
	HasSEPADebit           bool   `json:"has_sepa_debit"`
	HasPayPalSmartCheckout bool   `json:"has_paypal_smart_checkout"`
	AllowPartialPayments   bool   `json:"allow_partial_payments"`
}

// paymentOptionsBody is the wire payload EnablePaymentOptions sends for both
// invoices and invoice profiles. FreshBooks' /api/online-payments docs page
// gives one worked example, for the invoice variant:
//
//	{"gateway_name":"stripe","entity_id":2168250,"entity_type":"invoice","has_credit_card":true}
//
// entity_id is a bare JSON number there (the response later echoes it back
// quoted as a string, so a read model must not reuse this write type), and
// entity_type is singular ("invoice") despite the docs field table
// describing it as plural ("invoices") -- every request, response, and
// query example on the page agrees with the singular form, so that is what
// this library sends. The Postman example for the same endpoint omits both
// fields entirely; per CLAUDE.md's inferred-vs-confirmed rule the docs win
// (see spec section 3's STATE AS OF 2026-09-01 callout). The invoice-profile
// entity_type ("invoice_profile") has no docs or Postman example at all and
// is INFERRED by analogy.
type paymentOptionsBody struct {
	PaymentOptionsRequest
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
}

// EnablePaymentOptions turns on FreshBooks Payments for an invoice: which
// gateway to use, and which payment methods it accepts.
//
// inventory: Invoices/Enable Payment Options On Invoice
func (s *InvoicesService) EnablePaymentOptions(ctx context.Context, acct AccountID, id int64, req *PaymentOptionsRequest) error {
	if req == nil {
		return fmt.Errorf("freshbooks: Invoices.EnablePaymentOptions needs a request")
	}
	if err := pathSegment(string(acct)); err != nil {
		return err
	}
	path := fmt.Sprintf("/payments/account/%s/invoice/%d/payment_options", acct, id)
	body := paymentOptionsBody{PaymentOptionsRequest: *req, EntityType: "invoice", EntityID: id}
	return s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, nil)
}

// InvoicePresentationDefaults is the account's default branding, applied to
// a new invoice unless overridden.
//
// inventory: Invoices/Get default invoice presentation styles
func (s *InvoicesService) InvoicePresentationDefaults(ctx context.Context, acct AccountID) (*InvoicePresentation, error) {
	if err := pathSegment(string(acct)); err != nil {
		return nil, err
	}
	var resp struct {
		Presentation *InvoicePresentation `json:"presentation"`
	}
	path := fmt.Sprintf("/accounting/account/%s/invoices/presentations", acct)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Presentation, nil
}

func invoicesPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/invoices/invoices", acct), nil
}

func invoicePath(acct AccountID, id int64) (string, error) {
	base, err := invoicesPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}
