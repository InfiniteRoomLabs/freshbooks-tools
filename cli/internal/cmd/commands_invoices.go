package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// invoicesCommands wrap *freshbooks.InvoicesService.
var invoicesCommands = []Command{
	{
		Group: "invoices", Verb: "list",
		Short:   "List invoices",
		Service: "Invoices", Method: "List",
		Keys:  []string{"Invoices/List Invoices"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.InvoiceListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Invoices.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.Invoices.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "invoices", Verb: "get",
		Short:   "Get a single invoice",
		Service: "Invoices", Method: "Get",
		Keys:  []string{"Invoices/Single Invoice", "Invoices/Single Invoice w/ Logo"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true, HasInclude: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Invoices.Get(ctx, inv.Scope.AccountID, inv.IntID(), inv.IncludeOpt()...)
		},
	},
	{
		Group: "invoices", Verb: "create",
		Short:   "Create an invoice",
		Service: "Invoices", Method: "Create",
		Keys:  []string{"Invoices/Create Invoice with Expense", "Invoices/Single Invoice w/ Line Items", "Invoices/Single Invoice w/ Logo and styles", "Invoices/Single Invoice w/ Payment Gateway"},
		Class: ClassW, Scope: ScopeAccount, Body: true, HasInclude: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.InvoiceCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Invoices.Create(ctx, inv.Scope.AccountID, &body, inv.IncludeOpt()...)
		},
	},
	{
		Group: "invoices", Verb: "update",
		Short:   "Update an invoice",
		Service: "Invoices", Method: "Update",
		Keys:  []string{"Invoices/Update Invoice", "Invoices/Update Invoice w/ Expense", "Invoices/Toggle Online Payments on Invoice"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true, HasInclude: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.InvoiceUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Invoices.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body, inv.IncludeOpt()...)
		},
	},
	{
		Group: "invoices", Verb: "delete",
		Short:   "Delete an invoice",
		Service: "Invoices", Method: "Delete",
		Keys:  []string{"Invoices/Delete  Invoice"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Invoices.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
	{
		Group: "invoices", Verb: "send",
		Short:   "Send an invoice by email",
		Service: "Invoices", Method: "Send",
		Keys:  []string{"Invoices/Send Invoice by Email"},
		Class: ClassW, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.InvoiceSendRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return void(c.Invoices.Send(ctx, inv.Scope.AccountID, inv.IntID(), &body))
		},
	},
	{
		Group: "invoices", Verb: "pdf",
		Short:   "Download an invoice as a PDF",
		Service: "Invoices", Method: "PDF",
		Keys:  []string{"Invoices/Invoice Links/Downloads/Download Invoice PDF"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true, Binary: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Invoices.PDF(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "invoices", Verb: "share-link",
		Short:   "Get an invoice's public share link",
		Service: "Invoices", Method: "ShareLink",
		Keys:  []string{"Invoices/Invoice Links/Downloads/Share Link", "Invoices/Invoice Links/Downloads/Share PDF"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Invoices.ShareLink(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "invoices", Verb: "enable-payment-options",
		Short:   "Enable payment options on an invoice",
		Service: "Invoices", Method: "EnablePaymentOptions",
		Keys:  []string{"Invoices/Enable Payment Options On Invoice"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.PaymentOptionsRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return void(c.Invoices.EnablePaymentOptions(ctx, inv.Scope.AccountID, inv.IntID(), &body))
		},
	},
	{
		Group: "invoices", Verb: "invoice-presentation-defaults",
		Short:   "Get an account's default invoice presentation styles",
		Service: "Invoices", Method: "InvoicePresentationDefaults",
		Keys:  []string{"Invoices/Get default invoice presentation styles"},
		Class: ClassRO, Scope: ScopeAccount,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Invoices.InvoicePresentationDefaults(ctx, inv.Scope.AccountID)
		},
	},
}
