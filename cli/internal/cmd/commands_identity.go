package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/pflag"
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
		// F25/security A3-A4: this is the one moment a create actually
		// needs to show the generated client_secret -- there is no other
		// way to retrieve it later, since `applications` redacts it by
		// default -- so it is never redacted here.
		Group: "identity", Verb: "create-application",
		Short:   "Register a new developer application (prints the generated client_secret -- it is shown only here and on update)",
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
		// F25/security A3-A4: client_secret is redacted by default --
		// visible in `ps`/shell history via --client-secret elsewhere in
		// this CLI, and this list is the one place a caller could
		// otherwise dump every registered application's secret at once.
		Group: "identity", Verb: "applications",
		Short:   "List developer applications (client_secret redacted unless --show-secrets)",
		Service: "Identity", Method: "Applications",
		Keys:  []string{"Settings/Developer/Get all applications"},
		Class: ClassRO,
		ExtraFlags: func(fs *pflag.FlagSet) {
			fs.Bool("show-secrets", false, "include each application's client_secret in the output instead of redacting it")
		},
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			apps, err := c.Identity.Applications(ctx)
			if err != nil {
				return nil, err
			}
			show, _ := inv.Flags.GetBool("show-secrets")
			if !show {
				for i := range apps {
					apps[i].ClientSecret = ""
				}
			}
			return apps, nil
		},
	},
	{
		// F25/security A3-A4: same as create-application -- this is the
		// only other command that can legitimately need to show a
		// client_secret (rotating it), so it is never redacted here.
		Group: "identity", Verb: "update-application",
		Short:   "Update a developer application (prints its client_secret -- it is shown only here and on create)",
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
