package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// CreditNotesService is the client-credits resource (the "Clients/Credits"
// subfolder): goodwill credits, overpayment credits, and prepayment
// credits, all modeled by the API as one "credit_note" resource
// distinguished by CreditType.
type CreditNotesService struct{ client *Client }

// CreditNoteLine is one line item on a credit note.
type CreditNoteLine struct {
	// ID is set by FreshBooks; empty on a line you send to create one.
	ID string `json:"id,omitempty"`
	// Name and Description describe the line.
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// Qty and UnitCost determine the line's amount.
	Qty      string `json:"qty,omitempty"`
	UnitCost Money  `json:"unit_cost,omitempty"`
	// TaxName1, TaxAmount1, TaxName2, and TaxAmount2 record up to two taxes.
	TaxName1      string `json:"taxName1,omitempty"`
	TaxAmount1    Money  `json:"taxAmount1,omitempty"`
	TaxName2      string `json:"taxName2,omitempty"`
	TaxAmount2    Money  `json:"taxAmount2,omitempty"`
	CompoundedTax bool   `json:"compounded_tax,omitempty"`
}

// CreditNote is a credit issued to a client: a goodwill credit, a
// prepayment, or an overpayment carried forward.
type CreditNote struct {
	// ID and CreditID are the same value under two names; the API returns
	// both.
	ID       int64 `json:"id"`
	CreditID int64 `json:"creditid"`
	// ClientID is the client the credit belongs to.
	ClientID int64 `json:"clientid"`
	// CreditNumber is the account's human-readable credit number.
	CreditNumber string `json:"credit_number,omitempty"`
	// CreditType is "goodwill", "prepayment", or "overpayment".
	CreditType string `json:"credit_type,omitempty"`
	// CurrencyCode and Language are the credit's currency and language.
	CurrencyCode string `json:"currency_code,omitempty"`
	Language     string `json:"language,omitempty"`
	// CreateDate is when the credit was created.
	CreateDate Date `json:"create_date"`
	// Notes and Terms are free-text fields.
	Notes string `json:"notes,omitempty"`
	Terms string `json:"terms,omitempty"`
	// Amount is the credit's total; Paid is how much of it has been
	// applied.
	Amount Money `json:"amount,omitempty"`
	Paid   Money `json:"paid,omitempty"`
	// Status, DisplayStatus, and PaymentStatus describe the credit's state.
	Status        string `json:"status,omitempty"`
	DisplayStatus string `json:"display_status,omitempty"`
	PaymentStatus string `json:"payment_status,omitempty"`
	// FirstName, LastName, Organization, and the address fields snapshot
	// the client's details at credit-note time.
	FirstName    string `json:"fname,omitempty"`
	LastName     string `json:"lname,omitempty"`
	Organization string `json:"organization,omitempty"`
	Description  string `json:"description,omitempty"`
	Street       string `json:"street,omitempty"`
	Street2      string `json:"street2,omitempty"`
	City         string `json:"city,omitempty"`
	Province     string `json:"province,omitempty"`
	Code         string `json:"code,omitempty"`
	Country      string `json:"country,omitempty"`
	VatName      string `json:"vat_name,omitempty"`
	VatNumber    string `json:"vat_number,omitempty"`
	// Template names the presentation template.
	Template string `json:"template,omitempty"`
	// Lines are the credit note's line items.
	Lines []CreditNoteLine `json:"lines,omitempty"`
	// VisState is the credit note's visibility state.
	VisState VisState `json:"vis_state"`
}

type creditNoteEnvelope struct {
	CreditNote CreditNote `json:"credit_note"`
}

type creditNoteListEnvelope struct {
	CreditNotes []CreditNote `json:"credit_notes"`
	PageMeta
}

// CreditNoteWriteRequest is the payload for Create and Update. The Postman
// collection carries both twice, once for a goodwill credit and once for a
// prepayment credit; since both pairs differ only in CreditType and example
// values on this same request shape, each pair stacks on one method
// (Create, Update).
type CreditNoteWriteRequest struct {
	ClientID     int64            `json:"clientid,omitempty"`
	CreditNumber string           `json:"credit_number,omitempty"`
	CreditType   string           `json:"credit_type,omitempty"`
	CurrencyCode string           `json:"currency_code,omitempty"`
	Language     string           `json:"language,omitempty"`
	CreateDate   *Date            `json:"create_date,omitempty"`
	Notes        string           `json:"notes,omitempty"`
	Terms        string           `json:"terms,omitempty"`
	Lines        []CreditNoteLine `json:"lines,omitempty"`
}

// CreditNoteListOptions filters and paginates List.
type CreditNoteListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *CreditNoteListOptions) requestOptions() []RequestOption {
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

func creditNotesPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/credit_notes/credit_notes", acct)
}

func creditNotePath(acct AccountID, id int64) string {
	return fmt.Sprintf("/accounting/account/%s/credit_notes/credit_notes/%d", acct, id)
}

// List returns one page of credit notes.
//
// inventory: Clients/Credits/List Credits
func (s *CreditNotesService) List(ctx context.Context, acct AccountID, opts *CreditNoteListOptions) (*Page[CreditNote], error) {
	var env creditNoteListEnvelope
	if err := s.client.do(ctx, http.MethodGet, creditNotesPath(acct), FamilyAccounting, nil, &env, opts.requestOptions()...); err != nil {
		return nil, err
	}
	return &Page[CreditNote]{Items: env.CreditNotes, Page: env.Page, Pages: env.Pages, PerPage: env.PerPage, Total: env.Total}, nil
}

// All walks every page of credit notes, auto-paginating.
func (s *CreditNotesService) All(ctx context.Context, acct AccountID, opts *CreditNoteListOptions) iter.Seq2[CreditNote, error] {
	perPage := 100
	var search Search
	if opts != nil {
		if opts.PerPage > 0 {
			perPage = opts.PerPage
		}
		search = opts.Search
	}
	return All(ctx, func(ctx context.Context, page int) (*Page[CreditNote], error) {
		return s.List(ctx, acct, &CreditNoteListOptions{Search: search, Page: page, PerPage: perPage})
	})
}

// Create issues a new credit note. CreditType distinguishes a goodwill
// credit from a prepayment credit; both "Create Credit Note" and "Create
// Prepayment Credit" inventory keys stack on this one method.
//
// inventory: Clients/Credits/Create Credit Note
// inventory: Clients/Credits/Create Prepayment Credit
func (s *CreditNotesService) Create(ctx context.Context, acct AccountID, req *CreditNoteWriteRequest) (*CreditNote, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: CreditNotes.Create needs a request")
	}
	body := struct {
		CreditNote *CreditNoteWriteRequest `json:"credit_note"`
	}{req}
	var env creditNoteEnvelope
	if err := s.client.do(ctx, http.MethodPost, creditNotesPath(acct), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.CreditNote, nil
}

// Update changes an existing credit note. "Update Credit Note" and "Update
// Prepayment Credit" stack the same way Create's two keys do.
//
// inventory: Clients/Credits/Update Credit Note
// inventory: Clients/Credits/Update Prepayment Credit
func (s *CreditNotesService) Update(ctx context.Context, acct AccountID, id int64, req *CreditNoteWriteRequest) (*CreditNote, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: CreditNotes.Update needs a request")
	}
	body := struct {
		CreditNote *CreditNoteWriteRequest `json:"credit_note"`
	}{req}
	var env creditNoteEnvelope
	if err := s.client.do(ctx, http.MethodPut, creditNotePath(acct, id), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.CreditNote, nil
}

// Delete soft-deletes a credit note by setting vis_state to 1. FreshBooks
// models this as a PUT, not a real HTTP DELETE.
//
// inventory: Clients/Credits/Delete Credit
func (s *CreditNotesService) Delete(ctx context.Context, acct AccountID, id int64) error {
	body := map[string]any{"credit_note": map[string]any{"vis_state": VisStateDeleted}}
	return s.client.do(ctx, http.MethodPut, creditNotePath(acct, id), FamilyAccounting, body, nil)
}
