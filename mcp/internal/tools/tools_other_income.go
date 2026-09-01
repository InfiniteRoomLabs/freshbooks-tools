package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type otherIncomeCreateIn struct {
	AcctScope
	Body freshbooks.OtherIncomeCreateRequest `json:"body" jsonschema:"the other-income fields to create"`
}

type otherIncomeListIn struct {
	AcctScope
	listIn
}

type otherIncomeUpdateIn struct {
	AcctScope
	ID   int64                               `json:"id" jsonschema:"the other-income record's id"`
	Body freshbooks.OtherIncomeUpdateRequest `json:"body" jsonschema:"the other-income fields to update"`
}

type otherIncomeIDIn struct {
	AcctScope
	ID int64 `json:"id" jsonschema:"the other-income record's id"`
}

// otherIncomeSpecs are the tools wrapping *freshbooks.OtherIncomeService.
var otherIncomeSpecs = []Spec{
	newSpec("other_income_create",
		"Record other income for a business. See https://www.freshbooks.com/api/accounting.",
		"OtherIncome", "Create",
		[]string{"Accounting/Other Income/Create Single Other Income", "Invoices/Other Income/Create Single Other Income"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in otherIncomeCreateIn) (any, error) {
			return c.OtherIncome.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("other_income_list",
		"List a business's other-income records. See https://www.freshbooks.com/api/accounting.",
		"OtherIncome", "List",
		[]string{"Accounting/Other Income/List Other Income", "Invoices/Other Income/List Other Income"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in otherIncomeListIn) (any, error) {
			return c.OtherIncome.List(ctx, scope.AccountID, &freshbooks.OtherIncomeListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("other_income_update",
		"Update an other-income record. See https://www.freshbooks.com/api/accounting.",
		"OtherIncome", "Update",
		[]string{"Accounting/Other Income/Update Single Other Income", "Invoices/Other Income/Update Single Other Income"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in otherIncomeUpdateIn) (any, error) {
			return c.OtherIncome.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("other_income_delete",
		"Delete an other-income record. See https://www.freshbooks.com/api/accounting.",
		"OtherIncome", "Delete",
		[]string{"Accounting/Other Income/Delete Single Other Income", "Invoices/Other Income/Delete Single Other Income"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in otherIncomeIDIn) (any, error) {
			return c.OtherIncome.Delete(ctx, scope.AccountID, in.ID)
		}),
}
