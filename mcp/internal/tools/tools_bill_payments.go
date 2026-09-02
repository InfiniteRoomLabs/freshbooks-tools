package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type billPaymentsCreateIn struct {
	AcctScope
	Body freshbooks.BillPaymentCreateRequest `json:"body" jsonschema:"the bill payment fields to create"`
}

type billPaymentsUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.BillPaymentUpdateRequest `json:"body" jsonschema:"the bill payment fields to update"`
}

// billPaymentsSpecs are the tools wrapping *freshbooks.BillPaymentsService.
var billPaymentsSpecs = []Spec{
	newSpec("bill_payments_create",
		"Record a payment against a vendor bill. See https://www.freshbooks.com/api/bills.",
		"BillPayments", "Create",
		[]string{"Expenses/Bills (Beta)/Add Payment to Bill"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billPaymentsCreateIn) (any, error) {
			return c.BillPayments.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("bill_payments_update",
		"Update a payment recorded against a vendor bill. See https://www.freshbooks.com/api/bills.",
		"BillPayments", "Update",
		[]string{"Expenses/Bills (Beta)/Edit Payment to Bill"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billPaymentsUpdateIn) (any, error) {
			return c.BillPayments.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
}
