package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type invoiceProfilesListIn struct {
	AcctScope
	listIn
}

type invoiceProfilesGetIn struct {
	AcctScope
	idIn
	includeIn
}

type invoiceProfilesCreateIn struct {
	AcctScope
	Body freshbooks.InvoiceProfileCreateRequest `json:"body" jsonschema:"the recurring invoice profile fields to create"`
	includeIn
}

type invoiceProfilesUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.InvoiceProfileUpdateRequest `json:"body" jsonschema:"the recurring invoice profile fields to update"`
}

type invoiceProfilesIDIn struct {
	AcctScope
	idIn
}

type invoiceProfilesEnablePaymentOptionsIn struct {
	AcctScope
	idIn
	Body freshbooks.PaymentOptionsRequest `json:"body" jsonschema:"the payment options to enable on the invoice profile"`
}

// invoiceProfilesSpecs are the tools wrapping
// *freshbooks.InvoiceProfilesService.
var invoiceProfilesSpecs = []Spec{
	newSpec("invoice_profiles_list",
		"List an account's recurring-invoice profiles. See https://www.freshbooks.com/api/invoices.",
		"InvoiceProfiles", "List",
		[]string{"Invoices/Invoice Recurring Template/List Invoice Profiles"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoiceProfilesListIn) (any, error) {
			return c.InvoiceProfiles.List(ctx, scope.AccountID, &freshbooks.InvoiceProfileListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("invoice_profiles_get",
		"Get a single recurring-invoice profile. See https://www.freshbooks.com/api/invoices.",
		"InvoiceProfiles", "Get",
		[]string{"Invoices/Invoice Recurring Template/Get Single Invoice Profile"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoiceProfilesGetIn) (any, error) {
			return c.InvoiceProfiles.Get(ctx, scope.AccountID, in.ID, in.opts()...)
		}),
	newSpec("invoice_profiles_create",
		"Create a recurring-invoice profile, optionally with a time entry holder. See https://www.freshbooks.com/api/invoices.",
		"InvoiceProfiles", "Create",
		[]string{"Invoices/Invoice Recurring Template/Create Single Invoice Profile", "Invoices/Invoice Recurring Template/Create Single Invoice Profile w/ Time Entry Holder"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoiceProfilesCreateIn) (any, error) {
			return c.InvoiceProfiles.Create(ctx, scope.AccountID, &in.Body, in.opts()...)
		}),
	newSpec("invoice_profiles_update",
		"Update a recurring-invoice profile. See https://www.freshbooks.com/api/invoices.",
		"InvoiceProfiles", "Update",
		[]string{"Invoices/Invoice Recurring Template/Update Invoice Profile"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoiceProfilesUpdateIn) (any, error) {
			return c.InvoiceProfiles.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("invoice_profiles_delete",
		"Delete a recurring-invoice profile. See https://www.freshbooks.com/api/invoices.",
		"InvoiceProfiles", "Delete",
		[]string{"Invoices/Invoice Recurring Template/Delete  Invoice Profile"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoiceProfilesIDIn) (any, error) {
			return void(c.InvoiceProfiles.Delete(ctx, scope.AccountID, in.ID))
		}),
	newSpec("invoice_profiles_enable_payment_options",
		"Enable payment options on a recurring-invoice profile. See https://www.freshbooks.com/api/invoices.",
		"InvoiceProfiles", "EnablePaymentOptions",
		[]string{"Invoices/Invoice Recurring Template/Enable Payment Options On Invoice Profile"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in invoiceProfilesEnablePaymentOptionsIn) (any, error) {
			return void(c.InvoiceProfiles.EnablePaymentOptions(ctx, scope.AccountID, in.ID, &in.Body))
		}),
}
