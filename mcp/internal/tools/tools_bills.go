package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type billsListIn struct {
	AcctScope
	listIn
}

type billsCreateIn struct {
	AcctScope
	Body freshbooks.BillCreateRequest `json:"body" jsonschema:"the bill fields to create"`
}

type billsIDIn struct {
	AcctScope
	idIn
}

// billsSpecs are the tools wrapping *freshbooks.BillsService.
var billsSpecs = []Spec{
	newSpec("bills_list",
		"List a business's vendor bills. See https://www.freshbooks.com/api/bills.",
		"Bills", "List",
		[]string{"Expenses/Bills (Beta)/Get Bills"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billsListIn) (any, error) {
			return c.Bills.List(ctx, scope.AccountID, &freshbooks.BillListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("bills_create",
		"Add a bill from a vendor. See https://www.freshbooks.com/api/bills.",
		"Bills", "Create",
		[]string{"Expenses/Bills (Beta)/Add Bill from Vendor"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billsCreateIn) (any, error) {
			return c.Bills.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("bills_archive",
		"Archive a vendor bill. See https://www.freshbooks.com/api/bills.",
		"Bills", "Archive",
		[]string{"Expenses/Bills (Beta)/Archive Bill"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billsIDIn) (any, error) {
			return c.Bills.Archive(ctx, scope.AccountID, in.ID)
		}),
	newSpec("bills_delete",
		"Delete a vendor bill. See https://www.freshbooks.com/api/bills.",
		"Bills", "Delete",
		[]string{"Expenses/Bills (Beta)/Delete Bill"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billsIDIn) (any, error) {
			return void(c.Bills.Delete(ctx, scope.AccountID, in.ID))
		}),
}
