package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// These four inputs carry PCI-sensitive data (a full card number, CVV, and
// gateway API keys) and are subject to docs/phases/3/plan.md's written
// tokenization security constraint: never echo any of it into a result,
// an error, or a log. The generic error mapping in errors.go only ever
// formats the *lib's* returned error (which never carries card data or a
// bearer token, per freshbooks/errors.go), and none of the Call closures
// below format `in` with %v/%+v -- both hold structurally, not by review.

type paymentOptionsFBPayTokenizeIn struct {
	Body freshbooks.FBPayTokenizeRequest `json:"body" jsonschema:"the card fields to tokenize; never echoed back"`
}

type paymentOptionsStripeTokenizeIn struct {
	Body freshbooks.StripeTokenizeRequest `json:"body" jsonschema:"the card fields to tokenize; never echoed back"`
	// APIKey mirrors StripeTokenizeRequest.APIKey, which the lib type tags
	// json:"-" because the wire body carries it at the top level rather
	// than nested in cc_info (see freshbooks/payment_options.go).
	APIKey string `json:"api_key" jsonschema:"the Stripe publishable key from gateways_get's Stripe connection"`
}

type paymentOptionsStripeCreateSetupIntentIn struct {
	AcctScope
	PaymentMethod string `json:"payment_method" jsonschema:"the Stripe payment method id returned by payment_options_stripe_tokenize"`
}

type paymentOptionsSaveCreditCardIn struct {
	AcctScope
	Body freshbooks.SaveCreditCardRequest `json:"body" jsonschema:"the tokenized card and its access rules to save"`
}

// paymentOptionsSpecs are the tools wrapping
// *freshbooks.PaymentOptionsService.
var paymentOptionsSpecs = []Spec{
	newSpec("payment_options_fb_pay_tokenize",
		"Exchange a raw card for a one-time FBPay token, without the card number ever passing through the regular FreshBooks API host. Feed the returned token into payment_options_save_credit_card.",
		"PaymentOptions", "FBPayTokenize",
		[]string{"Tokenization/1. [FBPAY] - Create Payment Method"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentOptionsFBPayTokenizeIn) (any, error) {
			return c.PaymentOptions.FBPayTokenize(ctx, &in.Body)
		}),
	newSpec("payment_options_stripe_tokenize",
		"Exchange a raw card for a Stripe payment method, without the card number ever passing through the regular FreshBooks API host.",
		"PaymentOptions", "StripeTokenize",
		[]string{"Tokenization/1. [STRIPE] - Create Payment Method"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentOptionsStripeTokenizeIn) (any, error) {
			body := in.Body
			body.APIKey = in.APIKey
			return c.PaymentOptions.StripeTokenize(ctx, &body)
		}),
	newSpec("payment_options_stripe_create_setup_intent",
		"Exchange a Stripe payment method for the token payment_options_save_credit_card expects.",
		"PaymentOptions", "StripeCreateSetupIntent",
		[]string{"Tokenization/2. [STRIPE] - Create Setup Intent Using Payment Method Key"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentOptionsStripeCreateSetupIntentIn) (any, error) {
			return c.PaymentOptions.StripeCreateSetupIntent(ctx, scope.AccountID, in.PaymentMethod)
		}),
	newSpec("payment_options_save_credit_card",
		"Attach a tokenized card to the system for future charges.",
		"PaymentOptions", "SaveCreditCard",
		[]string{"Tokenization/2. [FBPAY] - Create Setup Intent Using Payment Method Key", "Tokenization/3. [STRIPE] - Save Payment Method to Recurring Profile"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentOptionsSaveCreditCardIn) (any, error) {
			return c.PaymentOptions.SaveCreditCard(ctx, scope.AccountID, &in.Body)
		}),
}
