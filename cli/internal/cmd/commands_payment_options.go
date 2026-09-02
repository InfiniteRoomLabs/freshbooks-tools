package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/pflag"
)

// paymentOptionsCommands wrap *freshbooks.PaymentOptionsService.
//
// fb-pay-tokenize and stripe-tokenize take no account scope at all
// (they post to paid.freshbooks.com, not the accounting API root -- see
// freshbooks/payment_options.go's doOnHost usage), unlike
// docs/phases/4/commands.md's "S, B" for both rows; the lib signature
// wins. stripe-create-setup-intent's "--rate/--payment-method" column is
// also a commands.md heuristic miss -- the method takes only a payment
// method key, no rate.
var paymentOptionsCommands = []Command{
	{
		Group: "payment-options", Verb: "fb-pay-tokenize",
		Short:   "Tokenize a card via FreshBooks Payments",
		Service: "PaymentOptions", Method: "FBPayTokenize",
		Keys:  []string{"Tokenization/1. [FBPAY] - Create Payment Method"},
		Class: ClassW, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.FBPayTokenizeRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.PaymentOptions.FBPayTokenize(ctx, &body)
		},
	},
	{
		Group: "payment-options", Verb: "stripe-tokenize",
		Short:   "Tokenize a card via Stripe",
		Service: "PaymentOptions", Method: "StripeTokenize",
		Keys:  []string{"Tokenization/1. [STRIPE] - Create Payment Method"},
		Class: ClassW, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.StripeTokenizeRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.PaymentOptions.StripeTokenize(ctx, &body)
		},
	},
	{
		Group: "payment-options", Verb: "stripe-create-setup-intent",
		Short:   "Create a Stripe setup intent for a tokenized payment method",
		Service: "PaymentOptions", Method: "StripeCreateSetupIntent",
		Keys:  []string{"Tokenization/2. [STRIPE] - Create Setup Intent Using Payment Method Key"},
		Class: ClassW, Scope: ScopeAccount,
		ExtraFlags: func(fs *pflag.FlagSet) {
			fs.String("payment-method", "", "the tokenized Stripe payment method key (required)")
		},
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			pm, _ := inv.Flags.GetString("payment-method")
			if pm == "" {
				return nil, newUsageError("--payment-method is required")
			}
			return c.PaymentOptions.StripeCreateSetupIntent(ctx, inv.Scope.AccountID, pm)
		},
	},
	{
		Group: "payment-options", Verb: "save-credit-card",
		Short:   "Save a tokenized credit card to a recurring profile",
		Service: "PaymentOptions", Method: "SaveCreditCard",
		Keys:  []string{"Tokenization/2. [FBPAY] - Create Setup Intent Using Payment Method Key", "Tokenization/3. [STRIPE] - Save Payment Method to Recurring Profile"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.SaveCreditCardRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.PaymentOptions.SaveCreditCard(ctx, inv.Scope.AccountID, &body)
		},
	},
}
