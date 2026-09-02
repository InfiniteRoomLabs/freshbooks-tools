package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// billPaymentsCommands wrap *freshbooks.BillPaymentsService.
var billPaymentsCommands = []Command{
	{
		Group: "bill-payments", Verb: "create",
		Short:   "Record a payment against a bill",
		Service: "BillPayments", Method: "Create",
		Keys:  []string{"Expenses/Bills (Beta)/Add Payment to Bill"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.BillPaymentCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.BillPayments.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "bill-payments", Verb: "update",
		Short:   "Update a payment against a bill",
		Service: "BillPayments", Method: "Update",
		Keys:  []string{"Expenses/Bills (Beta)/Edit Payment to Bill"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.BillPaymentUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.BillPayments.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
}
