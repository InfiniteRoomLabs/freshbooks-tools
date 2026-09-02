package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// paymentsCommands wrap *freshbooks.PaymentsService.
var paymentsCommands = []Command{
	{
		Group: "payments", Verb: "list",
		Short:   "List invoice payments",
		Service: "Payments", Method: "List",
		Keys:  []string{"Invoices/Payments/List Payments"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.PaymentListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Payments.All(ctx, inv.Scope.AccountID, opts))
			}
			return c.Payments.List(ctx, inv.Scope.AccountID, opts)
		},
	},
	{
		Group: "payments", Verb: "get",
		Short:   "Get a single payment",
		Service: "Payments", Method: "Get",
		Keys:  []string{"Invoices/Payments/Single Payment"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Payments.Get(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "payments", Verb: "create",
		Short:   "Record a payment against an invoice",
		Service: "Payments", Method: "Create",
		Keys:  []string{"Invoices/Payments/Make Payment"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.PaymentCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Payments.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "payments", Verb: "update",
		Short:   "Update a payment",
		Service: "Payments", Method: "Update",
		Keys:  []string{"Invoices/Payments/Update Payment"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.PaymentUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Payments.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "payments", Verb: "delete",
		Short:   "Delete a payment",
		Service: "Payments", Method: "Delete",
		Keys:  []string{"Invoices/Payments/Delete Payment"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Payments.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
	{
		Group: "payments", Verb: "create-checkout-link",
		Short:   "Create a checkout link",
		Service: "Payments", Method: "CreateCheckoutLink",
		Keys:  []string{"Invoices/Payments/Single Checkout Link"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.CheckoutLink
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Payments.CreateCheckoutLink(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "payments", Verb: "update-checkout-link",
		Short:   "Update a checkout link",
		Service: "Payments", Method: "UpdateCheckoutLink",
		Keys:  []string{"Invoices/Payments/Update Checkout Link"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, IDKind: "string", Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.CheckoutLink
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Payments.UpdateCheckoutLink(ctx, inv.Scope.AccountID, inv.StrID(), &body)
		},
	},
	{
		Group: "payments", Verb: "delete-checkout-link",
		Short:   "Delete a checkout link",
		Service: "Payments", Method: "DeleteCheckoutLink",
		Keys:  []string{"Invoices/Payments/Delete Checkout Link"},
		Class: ClassD, Scope: ScopeAccount, HasID: true, IDKind: "string",
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Payments.DeleteCheckoutLink(ctx, inv.Scope.AccountID, inv.StrID()))
		},
	},
	{
		Group: "payments", Verb: "update-checkout-link-gateway",
		Short:   "Update a checkout link's payment gateway",
		Service: "Payments", Method: "UpdateCheckoutLinkGateway",
		Keys:  []string{"Invoices/Payments/Update Checkout Link Payment Gateway"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, IDKind: "string", Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.PaymentOptionsRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return void(c.Payments.UpdateCheckoutLinkGateway(ctx, inv.Scope.AccountID, inv.StrID(), &body))
		},
	},
}
