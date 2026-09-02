package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// servicesCommands wrap *freshbooks.ServicesService, which mixes scope
// families within one service (Get/List are business-scoped,
// Create/GetBillableItem are account-scoped) -- CLAUDE.md's documented
// gotcha.
var servicesCommands = []Command{
	{
		Group: "services", Verb: "get",
		Short:   "Get a single service",
		Service: "Services", Method: "Get",
		Keys:  []string{"Settings/Items and Services/Get a Single Service"},
		Class: ClassRO, Scope: ScopeBusiness, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Services.Get(ctx, inv.Scope.BusinessID, inv.IntID())
		},
	},
	{
		Group: "services", Verb: "list",
		Short:   "List services",
		Service: "Services", Method: "List",
		Keys:  []string{"Settings/Items and Services/List Services"},
		Class: ClassRO, Scope: ScopeBusiness, List: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.ServiceListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			return c.Services.List(ctx, inv.Scope.BusinessID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "services", Verb: "create",
		Short:   "Create a billable service",
		Service: "Services", Method: "Create",
		Keys:  []string{"Settings/Items and Services/Create Service"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.BillableItemCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Services.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "services", Verb: "get-billable-item",
		Short:   "Get a single billable item",
		Service: "Services", Method: "GetBillableItem",
		Keys:  []string{"Settings/Items and Services/Single Service"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Services.GetBillableItem(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
}
