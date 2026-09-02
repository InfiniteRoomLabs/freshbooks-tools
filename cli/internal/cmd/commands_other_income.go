package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// otherIncomeCommands wrap *freshbooks.OtherIncomeService.
var otherIncomeCommands = []Command{
	{
		Group: "other-income", Verb: "create",
		Short:   "Record other income",
		Service: "OtherIncome", Method: "Create",
		Keys:  []string{"Accounting/Other Income/Create Single Other Income", "Invoices/Other Income/Create Single Other Income"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.OtherIncomeCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.OtherIncome.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "other-income", Verb: "list",
		Short:   "List other income",
		Service: "OtherIncome", Method: "List",
		Keys:  []string{"Accounting/Other Income/List Other Income", "Invoices/Other Income/List Other Income"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.OtherIncomeListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.OtherIncome.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.OtherIncome.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "other-income", Verb: "update",
		Short:   "Update other income",
		Service: "OtherIncome", Method: "Update",
		Keys:  []string{"Accounting/Other Income/Update Single Other Income", "Invoices/Other Income/Update Single Other Income"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.OtherIncomeUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.OtherIncome.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "other-income", Verb: "delete",
		Short:   "Delete other income",
		Service: "OtherIncome", Method: "Delete",
		Keys:  []string{"Accounting/Other Income/Delete Single Other Income", "Invoices/Other Income/Delete Single Other Income"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.OtherIncome.Delete(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
}
