package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type expensesListIn struct {
	AcctScope
	listIn
}

type expensesGetIn struct {
	AcctScope
	idIn
	includeIn
}

type expensesCreateIn struct {
	AcctScope
	Body freshbooks.ExpenseWriteRequest `json:"body" jsonschema:"the expense fields to create"`
}

type expensesUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.ExpenseWriteRequest `json:"body" jsonschema:"the expense fields to update"`
}

type expensesIDIn struct {
	AcctScope
	idIn
}

type expensesCreateRecurringIn struct {
	AcctScope
	Body freshbooks.ExpenseProfileCreateRequest `json:"body" jsonschema:"the recurring expense profile fields to create"`
}

// expensesSpecs are the tools wrapping *freshbooks.ExpensesService.
var expensesSpecs = []Spec{
	newSpec("expenses_list",
		"List an account's expenses. See https://www.freshbooks.com/api/expenses.",
		"Expenses", "List",
		[]string{"Expenses/List Expenses"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expensesListIn) (any, error) {
			return c.Expenses.List(ctx, scope.AccountID, &freshbooks.ExpenseListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("expenses_get",
		"Get a single expense. See https://www.freshbooks.com/api/expenses.",
		"Expenses", "Get",
		[]string{"Expenses/Single Expense"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expensesGetIn) (any, error) {
			return c.Expenses.Get(ctx, scope.AccountID, in.ID, in.opts()...)
		}),
	newSpec("expenses_create",
		"Create an expense, optionally with a receipt attached. See https://www.freshbooks.com/api/expenses.",
		"Expenses", "Create",
		[]string{"Expenses/Create Expense", "Expenses/Create Expense with Receipt"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expensesCreateIn) (any, error) {
			return c.Expenses.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("expenses_update",
		"Update an expense, optionally its receipt. See https://www.freshbooks.com/api/expenses.",
		"Expenses", "Update",
		[]string{"Expenses/Update Expense", "Expenses/Update Expense with Receipt"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expensesUpdateIn) (any, error) {
			return c.Expenses.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("expenses_delete",
		"Delete an expense. See https://www.freshbooks.com/api/expenses.",
		"Expenses", "Delete",
		[]string{"Expenses/Delete Expense"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expensesIDIn) (any, error) {
			return void(c.Expenses.Delete(ctx, scope.AccountID, in.ID))
		}),
	newSpec("expenses_summaries",
		"Summarize an account's expenses. See https://www.freshbooks.com/api/expenses.",
		"Expenses", "Summaries",
		[]string{"Expenses/Expense Summaries"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in AcctScope) (any, error) {
			return c.Expenses.Summaries(ctx, scope.AccountID)
		}),
	newSpec("expenses_vendors",
		"List the distinct vendor names an account has used on expenses. See https://www.freshbooks.com/api/expenses.",
		"Expenses", "Vendors",
		[]string{"Expenses/Expense Vendors"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in AcctScope) (any, error) {
			return c.Expenses.Vendors(ctx, scope.AccountID)
		}),
	newSpec("expenses_create_recurring",
		"Create a recurring expense profile. See https://www.freshbooks.com/api/expenses.",
		"Expenses", "CreateRecurring",
		[]string{"Expenses/Create Recurring Expense"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in expensesCreateRecurringIn) (any, error) {
			return c.Expenses.CreateRecurring(ctx, scope.AccountID, &in.Body)
		}),
}
