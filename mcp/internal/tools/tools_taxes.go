package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type taxesListIn struct {
	AcctScope
	listIn
}

type taxesGetIn struct {
	AcctScope
	idIn
	includeIn
}

type taxesCreateIn struct {
	AcctScope
	Body freshbooks.TaxCreateRequest `json:"body" jsonschema:"the tax rate fields to create"`
}

type taxesUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.TaxUpdateRequest `json:"body" jsonschema:"the tax rate fields to update"`
}

type taxesIDIn struct {
	AcctScope
	idIn
}

// taxesSpecs are the tools wrapping *freshbooks.TaxesService.
var taxesSpecs = []Spec{
	newSpec("taxes_list",
		"List an account's tax rates. See https://www.freshbooks.com/api/other_apis.",
		"Taxes", "List",
		[]string{"Expenses/List Taxes", "Accounting/Taxes/List Taxes", "Settings/Items and Services/List Taxes"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in taxesListIn) (any, error) {
			return c.Taxes.List(ctx, scope.AccountID, &freshbooks.TaxListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("taxes_get",
		"Get a single tax rate. See https://www.freshbooks.com/api/other_apis.",
		"Taxes", "Get",
		[]string{"Expenses/Single Tax (GET)", "Accounting/Taxes/Get Single Tax", "Settings/Items and Services/Single Tax (GET)"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in taxesGetIn) (any, error) {
			return c.Taxes.Get(ctx, scope.AccountID, in.ID, in.opts()...)
		}),
	newSpec("taxes_create",
		"Create a tax rate. See https://www.freshbooks.com/api/other_apis.",
		"Taxes", "Create",
		[]string{"Expenses/Create Single Tax", "Accounting/Taxes/Create Single Tax", "Settings/Items and Services/Create Single Tax"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in taxesCreateIn) (any, error) {
			return c.Taxes.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("taxes_update",
		"Update a tax rate. See https://www.freshbooks.com/api/other_apis.",
		"Taxes", "Update",
		[]string{"Expenses/Update Tax", "Accounting/Taxes/Update Single Tax", "Settings/Items and Services/Update Tax"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in taxesUpdateIn) (any, error) {
			return c.Taxes.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("taxes_delete",
		"Delete a tax rate. See https://www.freshbooks.com/api/other_apis.",
		"Taxes", "Delete",
		[]string{"Expenses/Single Tax (DELETE)", "Accounting/Taxes/Delete Single Tax", "Settings/Items and Services/Single Tax (DELETE)"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in taxesIDIn) (any, error) {
			return void(c.Taxes.Delete(ctx, scope.AccountID, in.ID))
		}),
}
