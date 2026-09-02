package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type itemsListIn struct {
	AcctScope
	listIn
}

type itemsIDIn struct {
	AcctScope
	idIn
}

type itemsCreateIn struct {
	AcctScope
	Body freshbooks.ItemCreateRequest `json:"body" jsonschema:"the catalogue item fields to create"`
}

type itemsUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.ItemUpdateRequest `json:"body" jsonschema:"the catalogue item fields to update"`
}

// itemsSpecs are the tools wrapping *freshbooks.ItemsService.
var itemsSpecs = []Spec{
	newSpec("items_list",
		"List an account's catalogue items and services, optionally filtered by SKU. See https://www.freshbooks.com/api/invoices.",
		"Items", "List",
		[]string{"Invoices/Items and Services/List Items", "Invoices/Items and Services/List Items Filtered by SKU", "Settings/Items and Services/List Items", "Settings/Items and Services/List Items Filtered by SKU"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in itemsListIn) (any, error) {
			return c.Items.List(ctx, scope.AccountID, &freshbooks.ItemListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("items_get",
		"Get a single catalogue item. See https://www.freshbooks.com/api/invoices.",
		"Items", "Get",
		[]string{"Invoices/Items and Services/Single Item", "Settings/Items and Services/Single Item"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in itemsIDIn) (any, error) {
			return c.Items.Get(ctx, scope.AccountID, in.ID)
		}),
	newSpec("items_create",
		"Create a catalogue item. See https://www.freshbooks.com/api/invoices.",
		"Items", "Create",
		[]string{"Invoices/Items and Services/Create Item", "Settings/Items and Services/Create Item"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in itemsCreateIn) (any, error) {
			return c.Items.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("items_update",
		"Update a catalogue item. See https://www.freshbooks.com/api/invoices.",
		"Items", "Update",
		[]string{"Invoices/Items and Services/Update Item", "Settings/Items and Services/Update Item"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in itemsUpdateIn) (any, error) {
			return c.Items.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("items_delete",
		"Delete a catalogue item. See https://www.freshbooks.com/api/invoices.",
		"Items", "Delete",
		[]string{"Invoices/Items and Services/Delete Item", "Settings/Items and Services/Delete Item"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in itemsIDIn) (any, error) {
			return void(c.Items.Delete(ctx, scope.AccountID, in.ID))
		}),
}
