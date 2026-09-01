package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// BillPaymentsService is the payments-against-bills resource. Bills are a
// Beta endpoint in the Postman collection; facts here are INFERRED from the
// collection's examples and the FreshBooks docs page, not live-verified.
type BillPaymentsService struct{ client *Client }

// BillPayment is one payment applied against a vendor bill.
type BillPayment struct {
	// ID is the payment's identifier.
	ID int64 `json:"id"`
	// BillID is the bill this payment applies to.
	BillID int64 `json:"billid"`
	// Amount is the amount paid.
	Amount Money `json:"amount"`
	// PaymentType names how the payment was made, e.g. "Check", "Credit",
	// "Cash".
	PaymentType string `json:"payment_type"`
	// PaidDate is the date the payment was made.
	PaidDate Date `json:"paid_date"`
	// Note is an optional payment note.
	Note string `json:"note,omitempty"`
	// MatchedWithExpense reports whether FreshBooks reconciled this payment
	// against an expense record.
	MatchedWithExpense bool `json:"matched_with_expense,omitempty"`
	// VisState is the payment's visibility state.
	VisState VisState `json:"vis_state"`
}

type billPaymentEnvelope struct {
	BillPayment BillPayment `json:"bill_payment"`
}

// BillPaymentCreateRequest is the payload for Create. BillID, Amount,
// PaymentType, and PaidDate are required by the API.
type BillPaymentCreateRequest struct {
	BillID      int64  `json:"billid"`
	Amount      Money  `json:"amount"`
	PaymentType string `json:"payment_type"`
	PaidDate    Date   `json:"paid_date"`
	Note        string `json:"note,omitempty"`
}

// BillPaymentUpdateRequest is the payload for Update. Only non-nil fields
// are sent.
type BillPaymentUpdateRequest struct {
	BillID      *int64  `json:"billid,omitempty"`
	Amount      *Money  `json:"amount,omitempty"`
	PaymentType *string `json:"payment_type,omitempty"`
	PaidDate    *Date   `json:"paid_date,omitempty"`
	Note        *string `json:"note,omitempty"`
}

func billPaymentsPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/bill_payments/bill_payments", acct), nil
}

func billPaymentPath(acct AccountID, id int64) (string, error) {
	base, err := billPaymentsPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}

// Create records a new payment against a bill.
//
// inventory: Expenses/Bills (Beta)/Add Payment to Bill
func (s *BillPaymentsService) Create(ctx context.Context, acct AccountID, req *BillPaymentCreateRequest) (*BillPayment, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: BillPayments.Create needs a request")
	}
	path, err := billPaymentsPath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		BillPayment *BillPaymentCreateRequest `json:"bill_payment"`
	}{req}
	var env billPaymentEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.BillPayment, nil
}

// Update changes an existing bill payment. Only non-nil fields on req are
// sent.
//
// inventory: Expenses/Bills (Beta)/Edit Payment to Bill
func (s *BillPaymentsService) Update(ctx context.Context, acct AccountID, id int64, req *BillPaymentUpdateRequest) (*BillPayment, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: BillPayments.Update needs a request")
	}
	path, err := billPaymentPath(acct, id)
	if err != nil {
		return nil, err
	}
	body := struct {
		BillPayment *BillPaymentUpdateRequest `json:"bill_payment"`
	}{req}
	var env billPaymentEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.BillPayment, nil
}
