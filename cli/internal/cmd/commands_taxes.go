package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// taxesCommands wrap *freshbooks.TaxesService.
var taxesCommands = []Command{
	{
		Group: "taxes", Verb: "list",
		Short:   "List tax rates",
		Service: "Taxes", Method: "List",
		Keys:  []string{"Expenses/List Taxes", "Accounting/Taxes/List Taxes", "Settings/Items and Services/List Taxes"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.TaxListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Taxes.All(ctx, inv.Scope.AccountID, opts))
			}
			return c.Taxes.List(ctx, inv.Scope.AccountID, opts)
		},
	},
	{
		Group: "taxes", Verb: "get",
		Short:   "Get a single tax rate",
		Service: "Taxes", Method: "Get",
		Keys:  []string{"Expenses/Single Tax (GET)", "Accounting/Taxes/Get Single Tax", "Settings/Items and Services/Single Tax (GET)"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Taxes.Get(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "taxes", Verb: "create",
		Short:   "Create a tax rate",
		Service: "Taxes", Method: "Create",
		Keys:  []string{"Expenses/Create Single Tax", "Accounting/Taxes/Create Single Tax", "Settings/Items and Services/Create Single Tax"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.TaxCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Taxes.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "taxes", Verb: "update",
		Short:   "Update a tax rate",
		Service: "Taxes", Method: "Update",
		Keys:  []string{"Expenses/Update Tax", "Accounting/Taxes/Update Single Tax", "Settings/Items and Services/Update Tax"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.TaxUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Taxes.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "taxes", Verb: "delete",
		Short:   "Delete a tax rate",
		Service: "Taxes", Method: "Delete",
		Keys:  []string{"Expenses/Single Tax (DELETE)", "Accounting/Taxes/Delete Single Tax", "Settings/Items and Services/Single Tax (DELETE)"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Taxes.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
