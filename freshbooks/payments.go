package freshbooks

import (
	"context"
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
	ID                 int64    `json:"id"`
	LogID              int64    `json:"logid"`
	AccountingSystemID string   `json:"accounting_systemid,omitempty"`
	ClientID           int64    `json:"clientid"`
	InvoiceID          int64    `json:"invoiceid"`
	Amount             Money    `json:"amount"`
	Date               Date     `json:"date"`
	Type               string   `json:"type"`
	Note               string   `json:"note,omitempty"`
	Gateway            *string  `json:"gateway,omitempty"`
	OrderID            *string  `json:"orderid,omitempty"`
	TransactionID      *string  `json:"transactionid,omitempty"`
	CreditID           *int64   `json:"creditid,omitempty"`
	OverpaymentID      *int64   `json:"overpaymentid,omitempty"`
	FromCredit         bool     `json:"from_credit"`
	Updated            DateTime `json:"updated,omitempty"`
	VisState           VisState `json:"vis_state"`
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
	var opts []RequestOption
	if len(o.Search) > 0 {
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

// List returns one page of payments.
//
// inventory: Invoices/Payments/List Payments
func (s *PaymentsService) List(ctx context.Context, acct AccountID, opts *PaymentListOptions, extra ...RequestOption) (*Page[Payment], error) {
	var resp paymentListResponse
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, paymentsPath(acct), FamilyAccounting, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return &Page[Payment]{Items: resp.Payments, Page: resp.Page, Pages: resp.Pages, PerPage: resp.PerPage, Total: resp.Total}, nil
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
	var resp paymentEnvelope
	if err := s.client.do(ctx, http.MethodGet, paymentPath(acct, id), FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Payment, nil
}

// PaymentCreateRequest is the payload for Create.
type PaymentCreateRequest struct {
	InvoiceID int64  `json:"invoiceid"`
	Amount    Money  `json:"amount"`
	Date      Date   `json:"date,omitempty"`
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
	var resp paymentEnvelope
	if err := s.client.do(ctx, http.MethodPost, paymentsPath(acct), FamilyAccounting, paymentWriteEnvelope{Payment: req}, &resp); err != nil {
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
	var resp paymentEnvelope
	if err := s.client.do(ctx, http.MethodPut, paymentPath(acct, id), FamilyAccounting, paymentWriteEnvelope{Payment: req}, &resp); err != nil {
		return nil, err
	}
	return resp.Payment, nil
}

// Delete soft-deletes a payment by setting its visibility state.
//
// inventory: Invoices/Payments/Delete Payment
func (s *PaymentsService) Delete(ctx context.Context, acct AccountID, id int64) error {
	return s.client.do(ctx, http.MethodPut, paymentPath(acct, id), FamilyAccounting, paymentWriteEnvelope{Payment: map[string]int{"vis_state": int(VisStateDeleted)}}, nil)
}

func paymentsPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/payments/payments", acct)
}

func paymentPath(acct AccountID, id int64) string {
	return fmt.Sprintf("%s/%d", paymentsPath(acct), id)
}

// CheckoutLinkTax is one tax line applied to a checkout link's total.
type CheckoutLinkTax struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

// CheckoutLink is a FreshBooks Payments hosted checkout page: a link a
// client can pay through without an invoice. The FreshBooks Postman
// collection has no captured response example for this resource, so the
// response is decoded with the same shape as the request; unmarshalled
// zero fields carry through if the live API responds with a different
// shape. Confirming this live is future work (see the package doc note in
// doc.go about INFERRED payments-family facts).
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

// CreateCheckoutLink creates a new hosted checkout link. FreshBooks' Postman
// collection names this "Single Checkout Link" because the response is one
// link, not a list; it is a create operation.
//
// inventory: Invoices/Payments/Single Checkout Link
func (s *PaymentsService) CreateCheckoutLink(ctx context.Context, acct AccountID, link *CheckoutLink) (*CheckoutLink, error) {
	if link == nil {
		return nil, fmt.Errorf("freshbooks: Payments.CreateCheckoutLink needs a checkout link")
	}
	var resp checkoutLinkEnvelope
	if err := s.client.do(ctx, http.MethodPost, checkoutLinksPath(acct), FamilyBusiness, link, &resp); err != nil {
		return nil, err
	}
	if resp.CheckoutLink == nil {
		return link, nil
	}
	return resp.CheckoutLink, nil
}

// UpdateCheckoutLink changes an existing checkout link.
//
// inventory: Invoices/Payments/Update Checkout Link
func (s *PaymentsService) UpdateCheckoutLink(ctx context.Context, acct AccountID, id string, link *CheckoutLink) (*CheckoutLink, error) {
	if link == nil {
		return nil, fmt.Errorf("freshbooks: Payments.UpdateCheckoutLink needs a checkout link")
	}
	var resp checkoutLinkEnvelope
	if err := s.client.do(ctx, http.MethodPut, checkoutLinkPath(acct, id), FamilyBusiness, link, &resp); err != nil {
		return nil, err
	}
	if resp.CheckoutLink == nil {
		return link, nil
	}
	return resp.CheckoutLink, nil
}

// DeleteCheckoutLink permanently removes a checkout link. Unlike invoices
// and payments, FreshBooks Payments exposes a real HTTP DELETE for this
// resource rather than a soft-delete field.
//
// inventory: Invoices/Payments/Delete Checkout Link
func (s *PaymentsService) DeleteCheckoutLink(ctx context.Context, acct AccountID, id string) error {
	return s.client.do(ctx, http.MethodDelete, checkoutLinkPath(acct, id), FamilyBusiness, nil, nil)
}

// UpdateCheckoutLinkGateway configures which FreshBooks Payments gateway and
// payment methods a checkout link accepts.
//
// inventory: Invoices/Payments/Update Checkout Link Payment Gateway
func (s *PaymentsService) UpdateCheckoutLinkGateway(ctx context.Context, acct AccountID, id string, req *PaymentOptionsRequest) error {
	if req == nil {
		return fmt.Errorf("freshbooks: Payments.UpdateCheckoutLinkGateway needs a request")
	}
	path := fmt.Sprintf("/payments/account/%s/checkout_link/%s/payment_options", acct, id)
	return s.client.do(ctx, http.MethodPost, path, FamilyBusiness, req, nil)
}

func checkoutLinksPath(acct AccountID) string {
	return fmt.Sprintf("/payments/account/%s/checkout-links", acct)
}

func checkoutLinkPath(acct AccountID, id string) string {
	return fmt.Sprintf("%s/%s", checkoutLinksPath(acct), id)
}
