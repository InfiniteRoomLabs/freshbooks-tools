package freshbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// tokenizationHost is where raw-card tokenization is posted -- a different
// host from the rest of the API. Neither tokenization endpoint has a public
// FreshBooks docs page; both host and shape are INFERRED from the Postman
// collection alone.
const tokenizationHost = "paid.freshbooks.com"

// FBPayTokenizeRequest is the payload for PaymentOptionsService.FBPayTokenize.
//
// This struct carries PCI-sensitive data (a full card number and CVV) in
// the clear. Never log it, persist it, or embed it in an error: %v and
// %+v render String below, which redacts both, but a caller that formats
// individual fields (fmt.Sprintf("%s", req.CardNumber)) bypasses that.
// Discard the value once FBPayTokenize returns.
type FBPayTokenizeRequest struct {
	Name        string `json:"name"`
	CardNumber  string `json:"card_number"`
	ExpiryMonth string `json:"expiry_month"`
	ExpiryYear  string `json:"expiry_year"`
	Email       string `json:"email"`
	CVV         string `json:"cvv"`
	PostalCode  string `json:"postal_code"`
	Country     string `json:"country"`
}

// String renders the request with the card number and CVV redacted.
func (r FBPayTokenizeRequest) String() string {
	return fmt.Sprintf("freshbooks.FBPayTokenizeRequest{Name: %q, CardNumber: redacted, CVV: redacted, ExpiryMonth: %q, ExpiryYear: %q}",
		r.Name, r.ExpiryMonth, r.ExpiryYear)
}

// StripeTokenizeRequest is the payload for PaymentOptionsService.StripeTokenize.
//
// This struct carries PCI-sensitive data (a full card number) and a Stripe
// API key in the clear. See FBPayTokenizeRequest's doc comment: the same
// handling rules apply.
type StripeTokenizeRequest struct {
	Name        string `json:"name"`
	CardNumber  string `json:"card_number"`
	ExpiryMonth string `json:"expiry_month"`
	ExpiryYear  string `json:"expiry_year"`
	// APIKey is the Stripe publishable key from GatewaysService.Get's
	// StripeConnection.PublishableKey. Tagged json:"-": the captured
	// request body sends it once, at the top level of the request (see
	// StripeTokenize), not nested inside cc_info alongside these other
	// fields.
	APIKey string `json:"-"`
}

// String renders the request with the card number and API key redacted.
func (r StripeTokenizeRequest) String() string {
	return fmt.Sprintf("freshbooks.StripeTokenizeRequest{Name: %q, CardNumber: redacted, ExpiryMonth: %q, ExpiryYear: %q, APIKey: redacted}",
		r.Name, r.ExpiryMonth, r.ExpiryYear)
}

// CreditCardAccess controls where a saved credit card may be used.
type CreditCardAccess struct {
	// System allows the card to be charged by the account owner directly.
	System bool `json:"system"`
	// Client allows the client who owns the card to use it themselves.
	Client bool `json:"client"`
	// InvoiceProfiles lists the recurring-invoice-profile IDs the card is
	// authorized for.
	InvoiceProfiles []int64 `json:"invoice_profiles"`
}

// CreditCardToken is one raw-card token to attach when saving a credit
// card. Set Token for an FBPay token (from FBPayTokenize's response) or
// PaymentMethodID for a Stripe payment method (from StripeTokenize's
// response); GatewayName says which.
type CreditCardToken struct {
	// Token is the FBPay tokenize response's cc_token, when GatewayName is
	// "fbpay".
	Token string `json:"token,omitempty"`
	// PaymentMethodID is the Stripe payment method id, when GatewayName is
	// "stripe".
	PaymentMethodID string `json:"payment_method_id,omitempty"`
	// GatewayName is "fbpay" or "stripe".
	GatewayName string `json:"gateway_name"`
	// IsPrimary marks this token as the card's default charge method. Not
	// omitempty: false is a meaningful, explicit choice for a toggle like
	// this, not an absent value.
	IsPrimary bool `json:"is_primary"`
}

// SaveCreditCardRequest is the payload for PaymentOptionsService.SaveCreditCard.
type SaveCreditCardRequest struct {
	// SavedToSystemID identifies where the card is saved (an invoice
	// profile or client record); FreshBooks assigns this if left empty.
	SavedToSystemID string `json:"saved_to_system_id,omitempty"`
	// CardHolderUserID is the FreshBooks user the card belongs to.
	CardHolderUserID int64 `json:"card_holder_user_id"`
	// Access controls where the saved card may be used.
	Access CreditCardAccess `json:"access"`
	// CreditCardTokens carries the raw-card token(s) to save.
	CreditCardTokens []CreditCardToken `json:"credit_card_tokens"`
}

// SavedCreditCardToken is one gateway's reference for a saved card, as
// FreshBooks echoes it back.
type SavedCreditCardToken struct {
	ID          string `json:"id"`
	GatewayName string `json:"gateway_name"`
}

// SavedCreditCard is a credit card FreshBooks has stored for future
// charges.
type SavedCreditCard struct {
	CreditCardUUID   string                 `json:"credit_card_uuid"`
	CardHolderUserID int64                  `json:"card_holder_user_id"`
	SavedToSystemID  string                 `json:"saved_to_system_id"`
	CardType         string                 `json:"card_type"`
	ExpiryYear       string                 `json:"expiry_year"`
	ExpiryMonth      string                 `json:"expiry_month"`
	LastFourDigits   string                 `json:"last_four_digits"`
	CardHolderName   string                 `json:"card_holder_name"`
	Access           CreditCardAccess       `json:"access"`
	CreditCardTokens []SavedCreditCardToken `json:"credit_card_tokens"`
}

// FBPayTokenize exchanges a raw card for a one-time FBPay token
// (cc_token), without ever sending the card number through the regular
// FreshBooks API host. Feed the returned token into SaveCreditCard as a
// CreditCardToken with GatewayName "fbpay".
//
// inventory: Tokenization/1. [FBPAY] - Create Payment Method
func (s *PaymentOptionsService) FBPayTokenize(ctx context.Context, req *FBPayTokenizeRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("freshbooks: PaymentOptions.FBPayTokenize needs a request")
	}
	body := struct {
		CCInfo *FBPayTokenizeRequest `json:"cc_info"`
	}{req}
	var resp struct {
		CCToken string `json:"cc_token"`
	}
	if err := s.client.doOnHost(ctx, http.MethodPost, tokenizationHost, "/gateway/fbpay/tokenize", FamilyBusiness, body, &resp); err != nil {
		return "", err
	}
	return resp.CCToken, nil
}

// StripeTokenize exchanges a raw card for a Stripe payment method, without
// ever sending the card number through the regular FreshBooks API host.
// Its response is INFERRED to mirror Stripe's own create-payment-method
// object (the Postman collection carries no example), so raw holds the
// payload unparsed; the "id" field within it (a "pm_..." string) is what
// StripeCreateSetupIntent expects as PaymentMethod.
//
// inventory: Tokenization/1. [STRIPE] - Create Payment Method
func (s *PaymentOptionsService) StripeTokenize(ctx context.Context, req *StripeTokenizeRequest) (raw json.RawMessage, err error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: PaymentOptions.StripeTokenize needs a request")
	}
	body := struct {
		CCInfo *StripeTokenizeRequest `json:"cc_info"`
		APIKey string                 `json:"api_key"`
	}{req, req.APIKey}
	if err := s.client.doOnHost(ctx, http.MethodPost, tokenizationHost, "/gateway/stripe/payment-method", FamilyBusiness, body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// StripeCreateSetupIntent exchanges a Stripe payment method (from
// StripeTokenize) for whatever token SaveCreditCard's CreditCardToken
// expects as PaymentMethodID. Its response is INFERRED -- the Postman
// collection carries no example -- so raw holds the payload unparsed.
//
// inventory: Tokenization/2. [STRIPE] - Create Setup Intent Using Payment Method Key
func (s *PaymentOptionsService) StripeCreateSetupIntent(ctx context.Context, acct AccountID, paymentMethod string) (raw json.RawMessage, err error) {
	if err := pathSegment(string(acct)); err != nil {
		return nil, err
	}
	path := "/payments/account/" + string(acct) + "/gateway/stripe/credit-card/token"
	body := struct {
		PaymentMethod string `json:"payment_method"`
	}{paymentMethod}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// SaveCreditCard attaches a tokenized card (from FBPayTokenize or
// StripeTokenize+StripeCreateSetupIntent) to the system for future charges.
// Postman lists this operation twice under different names -- once per
// gateway -- both POST to the same endpoint with the same body shape.
//
// inventory: Tokenization/2. [FBPAY] - Create Setup Intent Using Payment Method Key
// inventory: Tokenization/3. [STRIPE] - Save Payment Method to Recurring Profile
func (s *PaymentOptionsService) SaveCreditCard(ctx context.Context, acct AccountID, req *SaveCreditCardRequest) (*SavedCreditCard, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: PaymentOptions.SaveCreditCard needs a request")
	}
	if err := pathSegment(string(acct)); err != nil {
		return nil, err
	}
	path := "/payments/account/" + string(acct) + "/credit-card"
	body := struct {
		CreditCard *SaveCreditCardRequest `json:"credit_card"`
	}{req}
	var resp struct {
		CreditCard SavedCreditCard `json:"credit_card"`
	}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return &resp.CreditCard, nil
}
