package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// expensesCommands wrap *freshbooks.ExpensesService.
var expensesCommands = []Command{
	{
		Group: "expenses", Verb: "list",
		Short:   "List expenses",
		Service: "Expenses", Method: "List",
		Keys:  []string{"Expenses/List Expenses"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.ExpenseListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Expenses.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.Expenses.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "expenses", Verb: "get",
		Short:   "Get a single expense",
		Service: "Expenses", Method: "Get",
		Keys:  []string{"Expenses/Single Expense"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true, HasInclude: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Expenses.Get(ctx, inv.Scope.AccountID, inv.IntID(), inv.IncludeOpt()...)
		},
	},
	{
		Group: "expenses", Verb: "create",
		Short:   "Create an expense",
		Service: "Expenses", Method: "Create",
		Keys:  []string{"Expenses/Create Expense", "Expenses/Create Expense with Receipt"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ExpenseWriteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Expenses.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "expenses", Verb: "update",
		Short:   "Update an expense",
		Service: "Expenses", Method: "Update",
		Keys:  []string{"Expenses/Update Expense", "Expenses/Update Expense with Receipt"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ExpenseWriteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Expenses.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "expenses", Verb: "delete",
		Short:   "Delete an expense",
		Service: "Expenses", Method: "Delete",
		Keys:  []string{"Expenses/Delete Expense"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Expenses.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
	{
		Group: "expenses", Verb: "summaries",
		Short:   "Get expense summaries",
		Service: "Expenses", Method: "Summaries",
		Keys:  []string{"Expenses/Expense Summaries"},
		Class: ClassRO, Scope: ScopeAccount,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Expenses.Summaries(ctx, inv.Scope.AccountID)
		},
	},
	{
		Group: "expenses", Verb: "vendors",
		Short:   "List distinct expense vendor names",
		Service: "Expenses", Method: "Vendors",
		Keys:  []string{"Expenses/Expense Vendors"},
		Class: ClassRO, Scope: ScopeAccount,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Expenses.Vendors(ctx, inv.Scope.AccountID)
		},
	},
	{
		Group: "expenses", Verb: "create-recurring",
		Short:   "Create a recurring expense profile",
		Service: "Expenses", Method: "CreateRecurring",
		Keys:  []string{"Expenses/Create Recurring Expense"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ExpenseProfileCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Expenses.CreateRecurring(ctx, inv.Scope.AccountID, &body)
		},
	},
}
