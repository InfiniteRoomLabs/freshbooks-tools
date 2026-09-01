package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// EstimatesService is the estimates and proposals resource.
type EstimatesService struct{ client *Client }

// EstimateLine is one line item on an estimate.
type EstimateLine struct {
	// LineID and Updated are set by FreshBooks; empty on a line you send.
	LineID  string   `json:"lineid,omitempty"`
	Updated DateTime `json:"updated,omitempty"`
	// Type is a FreshBooks line-type code; 0 is a normal line.
	Type int `json:"type,omitempty"`
	// Name and Description describe the line.
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// Qty and UnitCost determine the line's amount.
	Qty      string `json:"qty,omitempty"`
	UnitCost Money  `json:"unit_cost,omitzero"`
	// Amount is computed by FreshBooks from Qty * UnitCost.
	Amount Money `json:"amount,omitzero"`
	// TaxName1, TaxAmount1, TaxName2, and TaxAmount2 record up to two taxes.
	TaxName1   string `json:"taxName1,omitempty"`
	TaxAmount1 Money  `json:"taxAmount1,omitzero"`
	TaxName2   string `json:"taxName2,omitempty"`
	TaxAmount2 Money  `json:"taxAmount2,omitzero"`
	// ExpenseID links this line to a billed-through expense, when set.
	ExpenseID int64 `json:"expenseid,omitempty"`
}

// Estimate is a price estimate or proposal sent to a client, which may later
// be converted to an invoice.
type Estimate struct {
	// ID and EstimateID are the same value under two names; the API returns
	// both.
	ID         int64 `json:"id"`
	EstimateID int64 `json:"estimateid"`
	// EstimateNumber is the account's human-readable estimate number.
	EstimateNumber string `json:"estimate_number,omitempty"`
	// CustomerID is the client this estimate was sent to.
	CustomerID int64 `json:"customerid"`
	// CreateDate is when the estimate was created.
	CreateDate Date `json:"create_date"`
	// CreatedAt is the account-local creation timestamp.
	CreatedAt DateTime `json:"created_at,omitempty"`
	// Updated is the account-local last-modified timestamp.
	Updated DateTime `json:"updated,omitempty"`
	// CurrencyCode and Language are the estimate's currency and language.
	CurrencyCode string `json:"currency_code,omitempty"`
	Language     string `json:"language,omitempty"`
	// Notes and Terms are free-text fields shown on the estimate.
	Notes string `json:"notes,omitempty"`
	Terms string `json:"terms,omitempty"`
	// PONumber is an optional client purchase-order reference.
	PONumber string `json:"po_number,omitempty"`
	// DiscountValue is a percentage discount applied to the estimate.
	DiscountValue string `json:"discount_value,omitempty"`
	// DiscountTotal is the discount amount FreshBooks computed.
	DiscountTotal Money `json:"discount_total,omitzero"`
	// Amount is the estimate's total.
	Amount Money `json:"amount,omitzero"`
	// Template names the presentation template.
	Template string `json:"template,omitempty"`
	// Status is a numeric estimate status; DisplayStatus and UIStatus are
	// FreshBooks' human-readable renderings of it (e.g. "draft", "sent",
	// "accepted").
	Status        int    `json:"status,omitempty"`
	DisplayStatus string `json:"display_status,omitempty"`
	UIStatus      string `json:"ui_status,omitempty"`
	ReplyStatus   string `json:"reply_status,omitempty"`
	// Accepted reports whether the client has accepted the estimate.
	Accepted bool `json:"accepted,omitempty"`
	// Invoiced reports whether the estimate has been converted to an
	// invoice.
	Invoiced bool `json:"invoiced,omitempty"`
	// RichProposal and RequireClientSignature enable the proposal
	// presentation and e-signature features.
	RichProposal           bool `json:"rich_proposal,omitempty"`
	RequireClientSignature bool `json:"require_client_signature,omitempty"`
	// OwnerID is the staff member who owns the estimate.
	OwnerID int64 `json:"ownerid,omitempty"`
	// Description is a free-text summary shown on the estimate.
	Description string `json:"description,omitempty"`
	// FirstName, LastName, Organization, and the address fields snapshot
	// the client's details at estimate time.
	FirstName    string `json:"fname,omitempty"`
	LastName     string `json:"lname,omitempty"`
	Organization string `json:"organization,omitempty"`
	Street       string `json:"street,omitempty"`
	Street2      string `json:"street2,omitempty"`
	City         string `json:"city,omitempty"`
	Province     string `json:"province,omitempty"`
	Code         string `json:"code,omitempty"`
	Country      string `json:"country,omitempty"`
	VatName      string `json:"vat_name,omitempty"`
	VatNumber    string `json:"vat_number,omitempty"`
	// Lines are the estimate's line items, present when requested via
	// Include("lines").
	Lines []EstimateLine `json:"lines,omitempty"`
	// VisState is the estimate's visibility state.
	VisState VisState `json:"vis_state"`
}

type estimateEnvelope struct {
	Estimate Estimate `json:"estimate"`
}

type estimateListEnvelope struct {
	Estimates []Estimate `json:"estimates"`
	PageMeta
}

// EstimatePresentation configures the rich-proposal presentation (theme,
// logo) for Create. INFERRED from the Postman example; the FreshBooks docs
// page for estimates does not document this object.
type EstimatePresentation struct {
	ThemePrimaryColor string `json:"theme_primary_color,omitempty"`
	ImageLogoSrc      string `json:"image_logo_src,omitempty"`
}

// EstimateWriteRequest is the payload for Create and Update. The Postman
// collection carries Create twice under different names ("...w/ Sections,
// Logos, and E-signature" and "...With Estimate Lines"), which differ only
// in which optional fields are set on this same shared request type, so
// both keys stack on Create.
type EstimateWriteRequest struct {
	CustomerID             int64                 `json:"customerid,omitempty"`
	CreateDate             *Date                 `json:"create_date,omitempty"`
	CurrencyCode           string                `json:"currency_code,omitempty"`
	Language               string                `json:"language,omitempty"`
	Notes                  string                `json:"notes,omitempty"`
	Terms                  string                `json:"terms,omitempty"`
	PONumber               string                `json:"po_number,omitempty"`
	DiscountValue          *float64              `json:"discount_value,omitempty"`
	Template               string                `json:"template,omitempty"`
	EstimateNumber         string                `json:"estimate_number,omitempty"`
	Description            string                `json:"description,omitempty"`
	OwnerID                *int64                `json:"ownerid,omitempty"`
	RichProposal           *bool                 `json:"rich_proposal,omitempty"`
	RequireClientSignature *bool                 `json:"require_client_signature,omitempty"`
	Presentation           *EstimatePresentation `json:"presentation,omitempty"`
	Lines                  []EstimateLine        `json:"lines,omitempty"`
}

// EstimateSendRequest is the payload for Send.
type EstimateSendRequest struct {
	EmailRecipients         []string                 `json:"email_recipients"`
	EstimateCustomizedEmail *EstimateCustomizedEmail `json:"estimate_customized_email,omitempty"`
}

// EstimateCustomizedEmail overrides the subject and body of the email Send
// dispatches.
type EstimateCustomizedEmail struct {
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

// EstimateListOptions filters and paginates List.
type EstimateListOptions struct {
	Search  Search
	Page    int
	PerPage int
	Include []string
}

func (o *EstimateListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	opts := listOpts(o.Search, o.Page, o.PerPage)
	if len(o.Include) > 0 {
		opts = append(opts, Include(o.Include...))
	}
	return opts
}

func estimatesPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/estimates/estimates", acct), nil
}

func estimatePath(acct AccountID, id int64) (string, error) {
	base, err := estimatesPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}

// List returns one page of estimates.
//
// inventory: Estimates/List Estimates
func (s *EstimatesService) List(ctx context.Context, acct AccountID, opts *EstimateListOptions, extra ...RequestOption) (*Page[Estimate], error) {
	path, err := estimatesPath(acct)
	if err != nil {
		return nil, err
	}
	var env estimateListEnvelope
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(env.Estimates, env.PageMeta), nil
}

// All walks every page of estimates, auto-paginating.
func (s *EstimatesService) All(ctx context.Context, acct AccountID, opts *EstimateListOptions, extra ...RequestOption) iter.Seq2[Estimate, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Estimate], error) {
		o := EstimateListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage, o.Include = opts.Search, opts.PerPage, opts.Include
		}
		o.PerPage = pageSize(o.PerPage)
		return s.List(ctx, acct, &o, extra...)
	})
}

// Get retrieves a single estimate.
//
// inventory: Estimates/Single Estimate
func (s *EstimatesService) Get(ctx context.Context, acct AccountID, id int64, opts ...RequestOption) (*Estimate, error) {
	path, err := estimatePath(acct, id)
	if err != nil {
		return nil, err
	}
	var env estimateEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, opts...); err != nil {
		return nil, err
	}
	return &env.Estimate, nil
}

// Create adds a new estimate.
//
// inventory: Estimates/Create Single Proposal w/ Sections, Logos, and E-signature
// inventory: Estimates/Single Estimate With Estimate Lines
func (s *EstimatesService) Create(ctx context.Context, acct AccountID, req *EstimateWriteRequest) (*Estimate, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Estimates.Create needs a request")
	}
	path, err := estimatesPath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		Estimate *EstimateWriteRequest `json:"estimate"`
	}{req}
	var env estimateEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Estimate, nil
}

// Update changes an existing estimate.
//
// inventory: Estimates/Update Estimate
func (s *EstimatesService) Update(ctx context.Context, acct AccountID, id int64, req *EstimateWriteRequest) (*Estimate, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Estimates.Update needs a request")
	}
	path, err := estimatePath(acct, id)
	if err != nil {
		return nil, err
	}
	body := struct {
		Estimate *EstimateWriteRequest `json:"estimate"`
	}{req}
	var env estimateEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Estimate, nil
}

// Delete soft-deletes an estimate by setting vis_state to 1. FreshBooks
// models this as a PUT, not a real HTTP DELETE, matching the FreshBooks
// docs page's own "Delete Single Estimate" section and every other
// soft-delete in this API family (bills, vendors, credit notes).
//
// inventory: Estimates/Delete Estimate
func (s *EstimatesService) Delete(ctx context.Context, acct AccountID, id int64) error {
	path, err := estimatePath(acct, id)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "estimate")
}

// Accept marks an estimate accepted on the client's behalf.
//
// inventory: Estimates/Accept Estimate
func (s *EstimatesService) Accept(ctx context.Context, acct AccountID, id int64) (*Estimate, error) {
	path, err := estimatePath(acct, id)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"estimate": map[string]any{"action_accept": true}}
	var env estimateEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Estimate, nil
}

// Send emails the estimate to its recipients.
//
// inventory: Estimates/Send Estimate by Email
func (s *EstimatesService) Send(ctx context.Context, acct AccountID, id int64, req *EstimateSendRequest) error {
	if req == nil || len(req.EmailRecipients) == 0 {
		return fmt.Errorf("freshbooks: Estimates.Send needs at least one recipient")
	}
	path, err := estimatePath(acct, id)
	if err != nil {
		return err
	}
	body := map[string]any{"estimate": map[string]any{
		"email_recipients":          req.EmailRecipients,
		"estimate_customized_email": req.EstimateCustomizedEmail,
		"action_email":              true,
	}}
	return s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, nil)
}
