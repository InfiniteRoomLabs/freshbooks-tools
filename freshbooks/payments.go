package freshbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// PaymentsService (declared in services.go) is the accounting
// invoice-payments resource, plus the FreshBooks Payments checkout-link
// resource that lives under the same Postman "Payments" subfolder. Payments
// (record of money received against an invoice) and checkout links (a
// hosted page a client pays through) are different FreshBooks resources
// sharing one work-order batch; their methods are grouped in this one file
// and service to match.

// Payment is a record of money received against an invoice.
type Payment struct {
	ID                 int64   `json:"id"`
	LogID              int64   `json:"logid"`
	AccountingSystemID string  `json:"accounting_systemid,omitempty"`
	ClientID           int64   `json:"clientid"`
	InvoiceID          int64   `json:"invoiceid"`
	Amount             Money   `json:"amount"`
	Date               Date    `json:"date"`
	Type               string  `json:"type"`
	Note               string  `json:"note,omitempty"`
	Gateway            *string `json:"gateway,omitempty"`
	OrderID            *string `json:"orderid,omitempty"`
	// TransactionID is the gateway's transaction id. FreshBooks' docs type
	// it int (deprecated field, but still typed); every captured example
	// has it null, so this was INFERRED as *string until QA caught it
	// against the docs -- it is *int64.
	TransactionID *int64   `json:"transactionid,omitempty"`
	CreditID      *int64   `json:"creditid,omitempty"`
	OverpaymentID *int64   `json:"overpaymentid,omitempty"`
	FromCredit    bool     `json:"from_credit"`
	Updated       DateTime `json:"updated,omitempty"`
	VisState      VisState `json:"vis_state"`
}

type paymentEnvelope struct {
	Payment *Payment `json:"payment"`
}

type paymentWriteEnvelope struct {
	Payment any `json:"payment"`
}

type paymentListResponse struct {
	Payments []Payment `json:"payments"`
	PageMeta
}

// PaymentListOptions selects and paginates List.
type PaymentListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *PaymentListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

// List returns one page of payments.
//
// inventory: Invoices/Payments/List Payments
func (s *PaymentsService) List(ctx context.Context, acct AccountID, opts *PaymentListOptions, extra ...RequestOption) (*Page[Payment], error) {
	path, err := paymentsPath(acct)
	if err != nil {
		return nil, err
	}
	var resp paymentListResponse
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.Payments, resp.PageMeta), nil
}

// All iterates every payment across every page.
func (s *PaymentsService) All(ctx context.Context, acct AccountID, opts *PaymentListOptions, extra ...RequestOption) iter.Seq2[Payment, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Payment], error) {
		o := PaymentListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		return s.List(ctx, acct, &o, extra...)
	})
}

// Get fetches one payment by id.
//
// inventory: Invoices/Payments/Single Payment
func (s *PaymentsService) Get(ctx context.Context, acct AccountID, id int64) (*Payment, error) {
	path, err := paymentPath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp paymentEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Payment, nil
}

// PaymentCreateRequest is the payload for Create.
type PaymentCreateRequest struct {
	InvoiceID int64  `json:"invoiceid"`
	Amount    Money  `json:"amount"`
	Date      Date   `json:"date,omitzero"`
	Type      string `json:"type,omitempty"`
	Note      string `json:"note,omitempty"`
}

// Create records a payment against an invoice.
//
// inventory: Invoices/Payments/Make Payment
func (s *PaymentsService) Create(ctx context.Context, acct AccountID, req *PaymentCreateRequest) (*Payment, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Payments.Create needs a request")
	}
	path, err := paymentsPath(acct)
	if err != nil {
		return nil, err
	}
	var resp paymentEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, paymentWriteEnvelope{Payment: req}, &resp); err != nil {
		return nil, err
	}
	return resp.Payment, nil
}

// PaymentUpdateRequest is the payload for Update. Every field is optional;
// only set ones are sent.
type PaymentUpdateRequest struct {
	Amount *Money  `json:"amount,omitempty"`
	Date   *Date   `json:"date,omitempty"`
	Type   *string `json:"type,omitempty"`
	Note   *string `json:"note,omitempty"`
}

// Update changes fields on an existing payment.
//
// inventory: Invoices/Payments/Update Payment
func (s *PaymentsService) Update(ctx context.Context, acct AccountID, id int64, req *PaymentUpdateRequest) (*Payment, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Payments.Update needs a request")
	}
	path, err := paymentPath(acct, id)
	if err != nil {
		return nil, err
	}
	var resp paymentEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, paymentWriteEnvelope{Payment: req}, &resp); err != nil {
		return nil, err
	}
	return resp.Payment, nil
}

// Delete soft-deletes a payment by setting its visibility state.
//
// inventory: Invoices/Payments/Delete Payment
func (s *PaymentsService) Delete(ctx context.Context, acct AccountID, id int64) error {
	path, err := paymentPath(acct, id)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "payment")
}

func paymentsPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/payments/payments", acct), nil
}

func paymentPath(acct AccountID, id int64) (string, error) {
	base, err := paymentsPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}

// CheckoutLinkTax is one tax line applied to a checkout link's total.
type CheckoutLinkTax struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// CheckoutLink is a FreshBooks Payments hosted checkout page: a link a
// client can pay through without an invoice. This is INFERRED, not
// CONFIRMED: the FreshBooks Postman collection has no captured response
// example for this resource, and the /api/online-payments docs page
// documents payment_options but not checkout-link fields. CreateCheckoutLink
// and UpdateCheckoutLink decode defensively (see decodeCheckoutLink) rather
// than assuming one response shape; a live-conformance pass should confirm
// the real shape.
type CheckoutLink struct {
	ID               string            `json:"id,omitempty"`
	ItemID           string            `json:"item_id"`
	ItemName         string            `json:"item_name,omitempty"`
	Amount           string            `json:"amount"`
	Currency         string            `json:"currency,omitempty"`
	Note             string            `json:"note,omitempty"`
	IsActive         bool              `json:"is_active"`
	SendAdminReceipt bool              `json:"send_admin_receipt,omitempty"`
	CreatedAt        string            `json:"created_at,omitempty"`
	Taxes            []CheckoutLinkTax `json:"taxes,omitempty"`
}

type checkoutLinkEnvelope struct {
	CheckoutLink *CheckoutLink `json:"checkout_link"`
}

// decodeCheckoutLink tries the enveloped shape ({"checkout_link": {...}})
// FreshBooks uses for most business-family single-resource responses, then
// falls back to a flat CheckoutLink object, since no captured example
// confirms which one this endpoint actually returns. If neither decode
// yields an id, it returns an error instead of silently handing the caller
// back their own request -- an empty or malformed response must not be
// mistaken for success.
func decodeCheckoutLink(raw json.RawMessage) (*CheckoutLink, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("freshbooks: checkout link response was empty")
	}
	var env checkoutLinkEnvelope
	if err := json.Unmarshal(raw, &env); err == nil && env.CheckoutLink != nil && env.CheckoutLink.ID != "" {
		return env.CheckoutLink, nil
	}
	var flat CheckoutLink
	if err := json.Unmarshal(raw, &flat); err == nil && flat.ID != "" {
		return &flat, nil
	}
	return nil, fmt.Errorf("freshbooks: checkout link response did not contain an id in either the enveloped or flat shape")
}

// CreateCheckoutLink creates a new hosted checkout link. FreshBooks' Postman
// collection names this "Single Checkout Link" because the response is one
// link, not a list; it is a create operation.
//
// inventory: Invoices/Payments/Single Checkout Link
func (s *PaymentsService) CreateCheckoutLink(ctx context.Context, acct AccountID, link *CheckoutLink) (*CheckoutLink, error) {
	if link == nil {
		return nil, fmt.Errorf("freshbooks: Payments.CreateCheckoutLink needs a checkout link")
	}
	path, err := checkoutLinksPath(acct)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, http.MethodPost, path, FamilyBusiness, link, &raw); err != nil {
		return nil, err
	}
	return decodeCheckoutLink(raw)
}

// UpdateCheckoutLink changes an existing checkout link.
//
// inventory: Invoices/Payments/Update Checkout Link
func (s *PaymentsService) UpdateCheckoutLink(ctx context.Context, acct AccountID, id string, link *CheckoutLink) (*CheckoutLink, error) {
	if link == nil {
		return nil, fmt.Errorf("freshbooks: Payments.UpdateCheckoutLink needs a checkout link")
	}
	path, err := checkoutLinkPath(acct, id)
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := s.client.do(ctx, http.MethodPut, path, FamilyBusiness, link, &raw); err != nil {
		return nil, err
	}
	return decodeCheckoutLink(raw)
}

// DeleteCheckoutLink permanently removes a checkout link. Unlike invoices
// and payments, FreshBooks Payments exposes a real HTTP DELETE for this
// resource rather than a soft-delete field.
//
// inventory: Invoices/Payments/Delete Checkout Link
func (s *PaymentsService) DeleteCheckoutLink(ctx context.Context, acct AccountID, id string) error {
	path, err := checkoutLinkPath(acct, id)
	if err != nil {
		return err
	}
	return s.client.do(ctx, http.MethodDelete, path, FamilyBusiness, nil, nil)
}

// checkoutLinkGatewayRequest is the wire payload for
// UpdateCheckoutLinkGateway. It embeds PaymentOptionsRequest so every
// payment-method boolean travels with it (unlike the entity fields below,
// FreshBooks' only captured example for this endpoint agrees with the
// invoice variant's docs on the boolean set), and adds entity_type/
// entity_id identifying which checkout link is being configured;
// UpdateCheckoutLinkGateway builds those two from its id argument rather
// than asking the caller to repeat it. Unlike paymentOptionsBody's
// entity_id (an invoice or invoice-profile id, always numeric), a
// checkout-link id is a string.
type checkoutLinkGatewayRequest struct {
	PaymentOptionsRequest
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}

// UpdateCheckoutLinkGateway configures which FreshBooks Payments gateway and
// payment methods a checkout link accepts.
//
// inventory: Invoices/Payments/Update Checkout Link Payment Gateway
func (s *PaymentsService) UpdateCheckoutLinkGateway(ctx context.Context, acct AccountID, id string, req *PaymentOptionsRequest) error {
	if req == nil {
		return fmt.Errorf("freshbooks: Payments.UpdateCheckoutLinkGateway needs a request")
	}
	if err := pathSegment(string(acct)); err != nil {
		return err
	}
	if err := pathSegment(id); err != nil {
		return err
	}
	path := fmt.Sprintf("/payments/account/%s/checkout_link/%s/payment_options", acct, id)
	body := checkoutLinkGatewayRequest{PaymentOptionsRequest: *req, EntityType: "checkout_link", EntityID: id}
	return s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, nil)
}

func checkoutLinksPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/payments/account/%s/checkout-links", acct), nil
}

func checkoutLinkPath(acct AccountID, id string) (string, error) {
	base, err := checkoutLinksPath(acct)
	if err != nil {
		return "", err
	}
	if err := pathSegment(id); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", base, id), nil
}
