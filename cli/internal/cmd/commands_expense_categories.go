package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// expenseCategoriesCommands wrap *freshbooks.ExpenseCategoriesService.
var expenseCategoriesCommands = []Command{
	{
		Group: "expense-categories", Verb: "list",
		Short:   "List expense categories",
		Service: "ExpenseCategories", Method: "List",
		Keys:  []string{"Expenses/List Expense Categories"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.ExpenseCategoryListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.ExpenseCategories.All(ctx, inv.Scope.AccountID, opts))
			}
			return c.ExpenseCategories.List(ctx, inv.Scope.AccountID, opts)
		},
	},
	{
		Group: "expense-categories", Verb: "get",
		Short:   "Get a single expense category",
		Service: "ExpenseCategories", Method: "Get",
		Keys:  []string{"Expenses/Single Expense Category"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.ExpenseCategories.Get(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "expense-categories", Verb: "create",
		Short:   "Create a custom expense category",
		Service: "ExpenseCategories", Method: "Create",
		Keys:  []string{"Expenses/Create Custom Expense Category"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ExpenseCategoryCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.ExpenseCategories.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
}
