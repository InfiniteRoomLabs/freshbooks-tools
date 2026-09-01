package freshbooks

import (
	"context"
	"net/http"
)

// GatewayPricing is one gateway connection's per-transaction fee schedule.
type GatewayPricing struct {
	TierID                 int    `json:"tier_id"`
	PercentACH             string `json:"percent_ach"`
	PercentNonAmex         string `json:"percent_non_amex"`
	PercentNonAmexWithCard string `json:"percent_non_amex_with_card"`
	PercentAmex            string `json:"percent_amex"`
	PercentAmexWithCard    string `json:"percent_amex_with_card"`
	PercentVirtualTerminal string `json:"percent_virtual_terminal"`
	PerTransactionFee      string `json:"per_transaction_fee"`
}

// FBPayConnection is the account's FreshBooks Payments (WePay) gateway
// connection.
type FBPayConnection struct {
	ID                  string         `json:"id"`
	AccountID           string         `json:"account_id"`
	VisaDebitAccepted   bool           `json:"visa_debit_accepted"`
	Pricing             GatewayPricing `json:"pricing"`
	BankTransferEnabled bool           `json:"bank_transfer_enabled"`
	TOSAccepted         bool           `json:"tos_accepted"`
	State               string         `json:"state"`
	Email               string         `json:"email"`
	ManageAccountURL    string         `json:"manage_account_url"`
	Currencies          []string       `json:"currencies"`
	Country             string         `json:"country"`
}

// StripeConnection is the account's Stripe gateway connection.
type StripeConnection struct {
	ID                  string   `json:"id"`
	GatewayUserID       string   `json:"gateway_user_id"`
	UserState           string   `json:"user_state"`
	Email               string   `json:"email"`
	Country             string   `json:"country"`
	Currencies          []string `json:"currencies"`
	PublishableKey      string   `json:"publishable_key"`
	BankTransferEnabled bool     `json:"bank_transfer_enabled"`
}

// GatewayConnection is one set of payment-gateway connections for the
// account. FreshBooks answers a one-element array of these; a nil gateway
// field means the account has not connected that gateway.
type GatewayConnection struct {
	FBPay  *FBPayConnection  `json:"fbpay"`
	Stripe *StripeConnection `json:"stripe"`
}

// Get returns acct's connected payment gateways. The Postman collection
// lists this one endpoint three times under different names: "Get
// Publishable Key" (Tokenization), "Gateway Details" (Settings/Businesses),
// and "List Gateways" (Settings/Gateways); all three map here.
//
// inventory: Tokenization/1a. [STRIPE] -  Get Publishable Key
// inventory: Settings/Businesses/Gateway Details
// inventory: Settings/Gateways/List Gateways
func (s *GatewaysService) Get(ctx context.Context, acct AccountID) ([]GatewayConnection, error) {
	path := "/payments/account/" + string(acct) + "/gateway"
	var resp struct {
		GatewayConnections []GatewayConnection `json:"gateway_connections"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, familyForPath(path), nil, &resp); err != nil {
		return nil, err
	}
	return resp.GatewayConnections, nil
}
