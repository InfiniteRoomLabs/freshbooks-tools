package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// invoiceProfilesCommands wrap *freshbooks.InvoiceProfilesService.
var invoiceProfilesCommands = []Command{
	{
		Group: "invoice-profiles", Verb: "list",
		Short:   "List recurring-invoice profiles",
		Service: "InvoiceProfiles", Method: "List",
		Keys:  []string{"Invoices/Invoice Recurring Template/List Invoice Profiles"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.InvoiceProfileListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.InvoiceProfiles.All(ctx, inv.Scope.AccountID, opts))
			}
			return c.InvoiceProfiles.List(ctx, inv.Scope.AccountID, opts)
		},
	},
	{
		Group: "invoice-profiles", Verb: "get",
		Short:   "Get a single recurring-invoice profile",
		Service: "InvoiceProfiles", Method: "Get",
		Keys:  []string{"Invoices/Invoice Recurring Template/Get Single Invoice Profile"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.InvoiceProfiles.Get(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "invoice-profiles", Verb: "create",
		Short:   "Create a recurring-invoice profile",
		Service: "InvoiceProfiles", Method: "Create",
		Keys:  []string{"Invoices/Invoice Recurring Template/Create Single Invoice Profile", "Invoices/Invoice Recurring Template/Create Single Invoice Profile w/ Time Entry Holder"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.InvoiceProfileCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.InvoiceProfiles.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "invoice-profiles", Verb: "update",
		Short:   "Update a recurring-invoice profile",
		Service: "InvoiceProfiles", Method: "Update",
		Keys:  []string{"Invoices/Invoice Recurring Template/Update Invoice Profile"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.InvoiceProfileUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.InvoiceProfiles.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "invoice-profiles", Verb: "delete",
		Short:   "Delete a recurring-invoice profile",
		Service: "InvoiceProfiles", Method: "Delete",
		Keys:  []string{"Invoices/Invoice Recurring Template/Delete  Invoice Profile"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.InvoiceProfiles.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
	{
		Group: "invoice-profiles", Verb: "enable-payment-options",
		Short:   "Enable payment options on a recurring-invoice profile",
		Service: "InvoiceProfiles", Method: "EnablePaymentOptions",
		Keys:  []string{"Invoices/Invoice Recurring Template/Enable Payment Options On Invoice Profile"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.PaymentOptionsRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return void(c.InvoiceProfiles.EnablePaymentOptions(ctx, inv.Scope.AccountID, inv.IntID(), &body))
		},
	},
}
