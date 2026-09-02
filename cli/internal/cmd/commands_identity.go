package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// identityCommands wrap *freshbooks.IdentityService.
//
// identity delete-business and delete-business-subscription address the
// business/account by scope alone -- the lib methods take only a
// BusinessID or AccountID, no separate resource id -- matching
// docs/phases/4/commands.md's own "S" (no ID) flags column for both rows.
var identityCommands = []Command{
	{
		Group: "identity", Verb: "me",
		Short:   "List the memberships of the identity behind the current token",
		Service: "Identity", Method: "Me",
		Keys:  []string{"Authorization/Identity Info Call", "Authorization/List User"},
		Class: ClassRO,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Identity.Me(ctx)
		},
	},
	{
		Group: "identity", Verb: "whoami",
		Short:   "Show the full identity behind the current token",
		Service: "Identity", Method: "Whoami",
		Class: ClassRO,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Identity.Whoami(ctx)
		},
	},
	{
		Group: "identity", Verb: "register",
		Short:   "Register a new FreshBooks user",
		Service: "Identity", Method: "Register",
		Keys:  []string{"Authorization/Register as a new user"},
		Class: ClassW, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.RegisterRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Identity.Register(ctx, &body)
		},
	},
	{
		Group: "identity", Verb: "add-business",
		Short:   "Add a business to the current identity",
		Service: "Identity", Method: "AddBusiness",
		Keys:  []string{"Settings/Businesses/Add Business"},
		Class: ClassW, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.BusinessCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Identity.AddBusiness(ctx, &body)
		},
	},
	{
		Group: "identity", Verb: "delete-business",
		Short:   "Delete a business",
		Service: "Identity", Method: "DeleteBusiness",
		Keys:  []string{"Settings/Businesses/Delete Business"},
		Class: ClassD, Scope: ScopeBusiness,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Identity.DeleteBusiness(ctx, inv.Scope.BusinessID))
		},
	},
	{
		Group: "identity", Verb: "delete-business-subscription",
		Short:   "Delete a business's subscription",
		Service: "Identity", Method: "DeleteBusinessSubscription",
		Keys:  []string{"Settings/Businesses/Delete Business - Subscription"},
		Class: ClassD, Scope: ScopeAccount,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Identity.DeleteBusinessSubscription(ctx, inv.Scope.AccountID))
		},
	},
	{
		Group: "identity", Verb: "provision-payments",
		Short:   "Provision FreshBooks Payments for an account",
		Service: "Identity", Method: "ProvisionPayments",
		Keys:  []string{"Settings/Businesses/Provision FreshBooks Payments"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.PaymentsProvisionRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return void(c.Identity.ProvisionPayments(ctx, inv.Scope.AccountID, &body))
		},
	},
	{
		Group: "identity", Verb: "create-application",
		Short:   "Register a new developer application",
		Service: "Identity", Method: "CreateApplication",
		Keys:  []string{"Settings/Developer/Create new application"},
		Class: ClassW, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ApplicationCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Identity.CreateApplication(ctx, &body)
		},
	},
	{
		Group: "identity", Verb: "applications",
		Short:   "List developer applications",
		Service: "Identity", Method: "Applications",
		Keys:  []string{"Settings/Developer/Get all applications"},
		Class: ClassRO,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Identity.Applications(ctx)
		},
	},
	{
		Group: "identity", Verb: "update-application",
		Short:   "Update a developer application",
		Service: "Identity", Method: "UpdateApplication",
		Keys:  []string{"Settings/Developer/Modify existing application"},
		Class: ClassI, HasID: true, IDKind: "string", IDName: "client-id", Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ApplicationUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Identity.UpdateApplication(ctx, inv.StrID(), &body)
		},
	},
}
