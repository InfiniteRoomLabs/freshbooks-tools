package freshbooks

import (
	"context"
	"encoding/json"
	"net/http"
)

// GatewayPricing is one gateway connection's per-transaction fee schedule.
// The ACH/promo fields are pointers because the captured example carries
// them as null; their populated type is INFERRED from sibling fields
// (amounts as decimal strings, tier ids as ints), not observed live.
type GatewayPricing struct {
	TierID                 int     `json:"tier_id"`
	PercentACH             string  `json:"percent_ach"`
	PercentNonAmex         string  `json:"percent_non_amex"`
	PercentNonAmexWithCard string  `json:"percent_non_amex_with_card"`
	PercentAmex            string  `json:"percent_amex"`
	PercentAmexWithCard    string  `json:"percent_amex_with_card"`
	PercentVirtualTerminal string  `json:"percent_virtual_terminal"`
	ACHTier1               *string `json:"ach_tier_1"`
	ACHTier2               *string `json:"ach_tier_2"`
	ACHTier3               *string `json:"ach_tier_3"`
	PerTransactionFee      string  `json:"per_transaction_fee"`
	DefaultPricingTierID   *int    `json:"default_pricing_tier_id"`
	PromoExpiryDate        *string `json:"promo_expiry_date"`
	MaxACHFee              *string `json:"max_ach_fee"`
}

// BankInfo is an FBPay connection's payout banking details.
type BankInfo struct {
	TotalPayout                string            `json:"total_payout"`
	NextPayoutDate             *string           `json:"next_payout_date"`
	BankName                   string            `json:"bank_name"`
	LastPaymentDate            string            `json:"last_payment_date"`
	LastPaymentAmount          string            `json:"last_payment_amount"`
	WithdrawalPeriod           string            `json:"withdrawal_period"`
	WithdrawalType             string            `json:"withdrawal_type"`
	WithdrawalSchedule         []json.RawMessage `json:"withdrawal_schedule"`
	IncomingPendingAmount      string            `json:"incoming_pending_amount"`
	OutgoingWithdrawalSchedule []json.RawMessage `json:"outgoing_withdrawal_schedule"`
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
	BankInfo            BankInfo       `json:"bank_info"`
	ManageAccountURL    string         `json:"manage_account_url"`
	Currencies          []string       `json:"currencies"`
	Country             string         `json:"country"`
	// ActionReasons is empty in every captured example, so its populated
	// element shape is unconfirmed.
	ActionReasons []json.RawMessage `json:"action_reasons"`
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
	// MaxACHFee is a bare number in the capture (unlike GatewayPricing's
	// MaxACHFee, which is a null, presumed-string field on a different
	// object -- these are two distinct captured keys with two distinct
	// observed shapes, not one field modeled twice).
	MaxACHFee int `json:"max_ach_fee"`
}

// StripeCapability is one Stripe capability flag on a unified connection.
type StripeCapability struct {
	Capability string `json:"capability"`
	IsActive   bool   `json:"is_active"`
}

// StripeUnifiedConnection is the account's Stripe connection under
// FreshBooks' newer "unified" onboarding. CONFIRMED live 2026-09-02 (Phase
// 7): a live account onboarded through FreshBooks Payments answers with
// "stripe": null and the whole connection under "stripe_unified" instead,
// and the response carries no "fbpay" key at all. StripeConnection (the
// older "stripe" shape, from the Postman capture) is kept because an
// account onboarded before the change still answers there.
type StripeUnifiedConnection struct {
	ID              string `json:"id"`
	StripeAccountID string `json:"stripe_account_id"`
	Country         string `json:"country"`
	PublishableKey  string `json:"publishable_key"`
	StripeEmail     string `json:"stripe_email"`
	AccountStatus   string `json:"account_status"`
	// AvailableOnboardingStrategies lists the onboarding flows FreshBooks
	// offers for this account, e.g. "hosted".
	AvailableOnboardingStrategies  []string `json:"available_onboarding_strategies"`
	StripeChargesEnabled           bool     `json:"stripe_charges_enabled"`
	StripePayoutsEnabled           bool     `json:"stripe_payouts_enabled"`
	StripePayoutsScheduleInterval  string   `json:"stripe_payouts_schedule_interval"`
	StripePayoutsScheduleDelayDays int      `json:"stripe_payouts_schedule_delay_days"`
	HasCurrentlyDueRequirements    bool     `json:"has_currently_due_requirements"`
	HasPendingVerifications        bool     `json:"has_pending_verifications"`
	OnboardingCompletionPercent    int      `json:"onboarding_completion_percent"`
	ChargesFirstEnabledAt          DateTime `json:"charges_first_enabled_at"`
	PayoutsFirstEnabledAt          DateTime `json:"payouts_first_enabled_at"`
	StripeTOSAcceptedDate          DateTime `json:"stripe_tos_accepted_date"`
	StripeAccountUpdatedAt         DateTime `json:"stripe_account_updated_at"`
	// StripeRequirementsCurrentDeadline and Configuration are null and {}
	// respectively in the live capture, so their populated shapes are
	// unconfirmed; kept raw rather than guessed.
	StripeRequirementsCurrentDeadline json.RawMessage    `json:"stripe_requirements_current_deadline"`
	Configuration                     json.RawMessage    `json:"configuration"`
	Capabilities                      []StripeCapability `json:"capabilities"`
}

// GatewayConnection is one set of payment-gateway connections for the
// account. FreshBooks answers a one-element array of these; a nil gateway
// field means the account has not connected that gateway.
type GatewayConnection struct {
	FBPay  *FBPayConnection  `json:"fbpay"`
	Stripe *StripeConnection `json:"stripe"`
	// StripeUnified carries the newer unified-onboarding Stripe connection.
	// A live account has exactly one of Stripe and StripeUnified populated.
	StripeUnified *StripeUnifiedConnection `json:"stripe_unified"`
	// PayPal is null in every captured example, so its populated shape is
	// unconfirmed; kept rather than dropped so an account with PayPal
	// connected does not look identical to one without it.
	PayPal json.RawMessage `json:"paypal"`
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
	if err := pathSegment(string(acct)); err != nil {
		return nil, err
	}
	path := "/payments/account/" + string(acct) + "/gateway"
	var resp struct {
		GatewayConnections []GatewayConnection `json:"gateway_connections"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return resp.GatewayConnections, nil
}
