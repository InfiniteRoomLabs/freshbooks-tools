package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type expenseCategoriesListIn struct {
	AcctScope
	listIn
}

type expenseCategoriesGetIn struct {
	AcctScope
	idIn
	includeIn
}

type expenseCategoriesCreateIn struct {
	AcctScope
	Body freshbooks.ExpenseCategoryCreateRequest `json:"body" jsonschema:"the expense category fields to create"`
}

// expenseCategoriesSpecs are the tools wrapping
// *freshbooks.ExpenseCategoriesService.
var expenseCategoriesSpecs = []Spec{
	newSpec("expense_categories_list",
		"List an account's expense categories. See https://www.freshbooks.com/api/expenses.",
		"ExpenseCategories", "List",
		[]string{"Expenses/List Expense Categories"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expenseCategoriesListIn) (any, error) {
			return c.ExpenseCategories.List(ctx, scope.AccountID, &freshbooks.ExpenseCategoryListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("expense_categories_get",
		"Get a single expense category. See https://www.freshbooks.com/api/expenses.",
		"ExpenseCategories", "Get",
		[]string{"Expenses/Single Expense Category"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expenseCategoriesGetIn) (any, error) {
			return c.ExpenseCategories.Get(ctx, scope.AccountID, in.ID, in.opts()...)
		}),
	newSpec("expense_categories_create",
		"Create a custom expense category. See https://www.freshbooks.com/api/expenses.",
		"ExpenseCategories", "Create",
		[]string{"Expenses/Create Custom Expense Category"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expenseCategoriesCreateIn) (any, error) {
			return c.ExpenseCategories.Create(ctx, scope.AccountID, &in.Body)
		}),
}
