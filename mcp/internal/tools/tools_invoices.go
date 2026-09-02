package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type invoicesListIn struct {
	AcctScope
	listIn
}

type invoicesGetIn struct {
	AcctScope
	idIn
	includeIn
}

type invoicesCreateIn struct {
	AcctScope
	Body freshbooks.InvoiceCreateRequest `json:"body" jsonschema:"the invoice fields to create"`
	includeIn
}

type invoicesUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.InvoiceUpdateRequest `json:"body" jsonschema:"the invoice fields to update"`
	includeIn
}

type invoicesIDIn struct {
	AcctScope
	idIn
}

type invoicesSendIn struct {
	AcctScope
	idIn
	Body freshbooks.InvoiceSendRequest `json:"body" jsonschema:"the email fields to send the invoice with"`
}

type invoicesEnablePaymentOptionsIn struct {
	AcctScope
	idIn
	Body freshbooks.PaymentOptionsRequest `json:"body" jsonschema:"the payment options to enable on the invoice"`
}

// invoicesSpecs are the tools wrapping *freshbooks.InvoicesService.
var invoicesSpecs = []Spec{
	newSpec("invoices_list",
		"List an account's invoices. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "List",
		[]string{"Invoices/List Invoices"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesListIn) (any, error) {
			return c.Invoices.List(ctx, scope.AccountID, &freshbooks.InvoiceListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("invoices_get",
		"Get a single invoice. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "Get",
		[]string{"Invoices/Single Invoice", "Invoices/Single Invoice w/ Logo"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesGetIn) (any, error) {
			return c.Invoices.Get(ctx, scope.AccountID, in.ID, in.opts()...)
		}),
	newSpec("invoices_create",
		"Create an invoice, optionally with expenses, line items, a logo and styles, or a payment gateway. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "Create",
		[]string{"Invoices/Create Invoice with Expense", "Invoices/Single Invoice w/ Line Items", "Invoices/Single Invoice w/ Logo and styles", "Invoices/Single Invoice w/ Payment Gateway"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesCreateIn) (any, error) {
			return c.Invoices.Create(ctx, scope.AccountID, &in.Body, in.opts()...)
		}),
	newSpec("invoices_update",
		"Update an invoice, including toggling online payments or its expenses. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "Update",
		[]string{"Invoices/Update Invoice", "Invoices/Update Invoice w/ Expense", "Invoices/Toggle Online Payments on Invoice"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesUpdateIn) (any, error) {
			return c.Invoices.Update(ctx, scope.AccountID, in.ID, &in.Body, in.opts()...)
		}),
	newSpec("invoices_delete",
		"Delete an invoice. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "Delete",
		[]string{"Invoices/Delete  Invoice"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesIDIn) (any, error) {
			return void(c.Invoices.Delete(ctx, scope.AccountID, in.ID))
		}),
	newSpec("invoices_send",
		"Send an invoice to the client by email. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "Send",
		[]string{"Invoices/Send Invoice by Email"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesSendIn) (any, error) {
			return void(c.Invoices.Send(ctx, scope.AccountID, in.ID, &in.Body))
		}),
	newSpec("invoices_pdf",
		"Download an invoice as a PDF, base64-encoded. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "PDF",
		[]string{"Invoices/Invoice Links/Downloads/Download Invoice PDF"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesIDIn) (any, error) {
			data, err := c.Invoices.PDF(ctx, scope.AccountID, in.ID)
			if err != nil {
				return nil, err
			}
			return newBinaryResult("application/pdf", data), nil
		}),
	newSpec("invoices_share_link",
		"Get an invoice's public share link and PDF share link. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "ShareLink",
		[]string{"Invoices/Invoice Links/Downloads/Share Link", "Invoices/Invoice Links/Downloads/Share PDF"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesIDIn) (any, error) {
			return c.Invoices.ShareLink(ctx, scope.AccountID, in.ID)
		}),
	newSpec("invoices_enable_payment_options",
		"Enable payment options on an invoice. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "EnablePaymentOptions",
		[]string{"Invoices/Enable Payment Options On Invoice"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoicesEnablePaymentOptionsIn) (any, error) {
			return void(c.Invoices.EnablePaymentOptions(ctx, scope.AccountID, in.ID, &in.Body))
		}),
	newSpec("invoices_invoice_presentation_defaults",
		"Get an account's default invoice presentation styles. See https://www.freshbooks.com/api/invoices.",
		"Invoices", "InvoicePresentationDefaults",
		[]string{"Invoices/Get default invoice presentation styles"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in AcctScope) (any, error) {
			return c.Invoices.InvoicePresentationDefaults(ctx, scope.AccountID)
		}),
}
