package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type paymentsListIn struct {
	AcctScope
	listIn
}

type paymentsIDIn struct {
	AcctScope
	idIn
}

type paymentsCreateIn struct {
	AcctScope
	Body freshbooks.PaymentCreateRequest `json:"body" jsonschema:"the payment fields to create"`
}

type paymentsUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.PaymentUpdateRequest `json:"body" jsonschema:"the payment fields to update"`
}

type paymentsCreateCheckoutLinkIn struct {
	AcctScope
	Body freshbooks.CheckoutLink `json:"body" jsonschema:"the checkout link fields to create"`
}

type paymentsCheckoutLinkIDIn struct {
	AcctScope
	ID string `json:"id" jsonschema:"the checkout link id"`
}

type paymentsUpdateCheckoutLinkIn struct {
	AcctScope
	ID   string                  `json:"id" jsonschema:"the checkout link id"`
	Body freshbooks.CheckoutLink `json:"body" jsonschema:"the checkout link's replacement fields"`
}

type paymentsUpdateCheckoutLinkGatewayIn struct {
	AcctScope
	ID   string                           `json:"id" jsonschema:"the checkout link id"`
	Body freshbooks.PaymentOptionsRequest `json:"body" jsonschema:"the payment gateway options to set on the checkout link"`
}

// paymentsSpecs are the tools wrapping *freshbooks.PaymentsService.
var paymentsSpecs = []Spec{
	newSpec("payments_list",
		"List an account's invoice payments. See https://www.freshbooks.com/api/invoices.",
		"Payments", "List",
		[]string{"Invoices/Payments/List Payments"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsListIn) (any, error) {
			return c.Payments.List(ctx, scope.AccountID, &freshbooks.PaymentListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("payments_get",
		"Get a single invoice payment. See https://www.freshbooks.com/api/invoices.",
		"Payments", "Get",
		[]string{"Invoices/Payments/Single Payment"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsIDIn) (any, error) {
			return c.Payments.Get(ctx, scope.AccountID, in.ID)
		}),
	newSpec("payments_create",
		"Record a payment against an invoice. See https://www.freshbooks.com/api/invoices.",
		"Payments", "Create",
		[]string{"Invoices/Payments/Make Payment"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsCreateIn) (any, error) {
			return c.Payments.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("payments_update",
		"Update an invoice payment. See https://www.freshbooks.com/api/invoices.",
		"Payments", "Update",
		[]string{"Invoices/Payments/Update Payment"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsUpdateIn) (any, error) {
			return c.Payments.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("payments_delete",
		"Delete an invoice payment. See https://www.freshbooks.com/api/invoices.",
		"Payments", "Delete",
		[]string{"Invoices/Payments/Delete Payment"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsIDIn) (any, error) {
			return void(c.Payments.Delete(ctx, scope.AccountID, in.ID))
		}),
	newSpec("payments_create_checkout_link",
		"Create a hosted checkout link. See https://www.freshbooks.com/api/invoices.",
		"Payments", "CreateCheckoutLink",
		[]string{"Invoices/Payments/Single Checkout Link"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsCreateCheckoutLinkIn) (any, error) {
			return c.Payments.CreateCheckoutLink(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("payments_update_checkout_link",
		"Update a hosted checkout link. See https://www.freshbooks.com/api/invoices.",
		"Payments", "UpdateCheckoutLink",
		[]string{"Invoices/Payments/Update Checkout Link"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsUpdateCheckoutLinkIn) (any, error) {
			return c.Payments.UpdateCheckoutLink(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("payments_delete_checkout_link",
		"Delete a hosted checkout link. See https://www.freshbooks.com/api/invoices.",
		"Payments", "DeleteCheckoutLink",
		[]string{"Invoices/Payments/Delete Checkout Link"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsCheckoutLinkIDIn) (any, error) {
			return void(c.Payments.DeleteCheckoutLink(ctx, scope.AccountID, in.ID))
		}),
	newSpec("payments_update_checkout_link_gateway",
		"Update a hosted checkout link's payment gateway. See https://www.freshbooks.com/api/invoices.",
		"Payments", "UpdateCheckoutLinkGateway",
		[]string{"Invoices/Payments/Update Checkout Link Payment Gateway"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in paymentsUpdateCheckoutLinkGatewayIn) (any, error) {
			return void(c.Payments.UpdateCheckoutLinkGateway(ctx, scope.AccountID, in.ID, &in.Body))
		}),
}
