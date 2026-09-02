package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// billsCommands wrap *freshbooks.BillsService.
var billsCommands = []Command{
	{
		Group: "bills", Verb: "list",
		Short:   "List a business's vendor bills",
		Service: "Bills", Method: "List",
		Keys:  []string{"Expenses/Bills (Beta)/Get Bills"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.BillListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Bills.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.Bills.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "bills", Verb: "create",
		Short:   "Add a bill from a vendor",
		Service: "Bills", Method: "Create",
		Keys:  []string{"Expenses/Bills (Beta)/Add Bill from Vendor"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.BillCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Bills.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "bills", Verb: "archive",
		Short:   "Archive a vendor bill",
		Service: "Bills", Method: "Archive",
		Keys:  []string{"Expenses/Bills (Beta)/Archive Bill"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Bills.Archive(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "bills", Verb: "delete",
		Short:   "Delete a vendor bill",
		Service: "Bills", Method: "Delete",
		Keys:  []string{"Expenses/Bills (Beta)/Delete Bill"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Bills.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
