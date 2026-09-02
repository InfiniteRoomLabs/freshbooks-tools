package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/pflag"
)

// callbacksCommands wrap *freshbooks.CallbacksService.
var callbacksCommands = []Command{
	{
		Group: "callbacks", Verb: "register",
		Short:   "Register a webhook callback",
		Service: "Callbacks", Method: "Register",
		Keys:  []string{"Webhooks/Register for Callback"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.CallbackRegisterRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Callbacks.Register(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "callbacks", Verb: "list",
		Short:   "List webhook callbacks",
		Service: "Callbacks", Method: "List",
		Keys:  []string{"Webhooks/List Webhook Callbacks"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.CallbackListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Callbacks.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.Callbacks.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "callbacks", Verb: "delete",
		Short:   "Delete a webhook callback",
		Service: "Callbacks", Method: "Delete",
		Keys:  []string{"Webhooks/Delete Webhook Callback"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Callbacks.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
	{
		Group: "callbacks", Verb: "verify",
		Short:   "Verify a webhook callback",
		Service: "Callbacks", Method: "Verify",
		Keys:  []string{"Webhooks/Verify Webhook Callback"},
		Class: ClassI, Scope: ScopeAccount, HasID: true,
		ExtraFlags: func(fs *pflag.FlagSet) {
			fs.String("verifier", "", "the verifier code FreshBooks sent (required)")
		},
		RequiredFlags: []string{"verifier"},
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Callbacks.Verify(ctx, inv.Scope.AccountID, inv.IntID(), inv.RequiredString("verifier"))
		},
	},
	{
		Group: "callbacks", Verb: "resend-verification",
		Short:   "Resend a webhook callback's verification code",
		Service: "Callbacks", Method: "ResendVerification",
		Keys:  []string{"Webhooks/Resend Verification Code"},
		Class: ClassW, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Callbacks.ResendVerification(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
