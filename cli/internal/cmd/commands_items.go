package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// itemsCommands wrap *freshbooks.ItemsService.
var itemsCommands = []Command{
	{
		Group: "items", Verb: "list",
		Short:   "List catalogue items",
		Service: "Items", Method: "List",
		Keys:  []string{"Invoices/Items and Services/List Items", "Invoices/Items and Services/List Items Filtered by SKU", "Settings/Items and Services/List Items", "Settings/Items and Services/List Items Filtered by SKU"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.ItemListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Items.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.Items.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "items", Verb: "get",
		Short:   "Get a single catalogue item",
		Service: "Items", Method: "Get",
		Keys:  []string{"Invoices/Items and Services/Single Item", "Settings/Items and Services/Single Item"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Items.Get(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "items", Verb: "create",
		Short:   "Create a catalogue item",
		Service: "Items", Method: "Create",
		Keys:  []string{"Invoices/Items and Services/Create Item", "Settings/Items and Services/Create Item"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ItemCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Items.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "items", Verb: "update",
		Short:   "Update a catalogue item",
		Service: "Items", Method: "Update",
		Keys:  []string{"Invoices/Items and Services/Update Item", "Settings/Items and Services/Update Item"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ItemUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Items.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "items", Verb: "delete",
		Short:   "Delete a catalogue item",
		Service: "Items", Method: "Delete",
		Keys:  []string{"Invoices/Items and Services/Delete Item", "Settings/Items and Services/Delete Item"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Items.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
