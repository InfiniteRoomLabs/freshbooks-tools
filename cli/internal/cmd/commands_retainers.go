package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// retainersCommands wrap *freshbooks.RetainersService.
//
// RetainerListOptions has only a Search field (no Page/PerPage), per
// CLAUDE.md's gotcha note -- retainers list registers only --search, not
// --page/--per-page (NoPaging).
var retainersCommands = []Command{
	{
		Group: "retainers", Verb: "list",
		Short:   "List retainers",
		Service: "Retainers", Method: "List",
		Keys:  []string{"Invoices/Retainers/Get all retainers"},
		Class: ClassRO, Scope: ScopeBusiness, List: true, NoPaging: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.RetainerListOptions{Search: inv.Search()}
			return c.Retainers.List(ctx, inv.Scope.BusinessID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "retainers", Verb: "get",
		Short:   "Get a single retainer",
		Service: "Retainers", Method: "Get",
		Keys:  []string{"Invoices/Retainers/Single Retainer"},
		Class: ClassRO, Scope: ScopeBusiness, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Retainers.Get(ctx, inv.Scope.BusinessID, inv.IntID())
		},
	},
	{
		Group: "retainers", Verb: "create",
		Short:   "Create a retainer",
		Service: "Retainers", Method: "Create",
		Keys:  []string{"Invoices/Retainers/Create Retainer"},
		Class: ClassW, Scope: ScopeBusiness, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.RetainerCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Retainers.Create(ctx, inv.Scope.BusinessID, &body)
		},
	},
	{
		Group: "retainers", Verb: "update",
		Short:   "Update a retainer",
		Service: "Retainers", Method: "Update",
		Keys:  []string{"Invoices/Retainers/Update Retainer"},
		Class: ClassI, Scope: ScopeBusiness, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.RetainerUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Retainers.Update(ctx, inv.Scope.BusinessID, inv.IntID(), &body)
		},
	},
	{
		Group: "retainers", Verb: "delete",
		Short:   "Delete a retainer",
		Service: "Retainers", Method: "Delete",
		Keys:  []string{"Invoices/Retainers/Delete Retainer"},
		Class: ClassD, Scope: ScopeBusiness, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Retainers.Delete(ctx, inv.Scope.BusinessID, inv.IntID())
		},
	},
	{
		Group: "retainers", Verb: "undelete",
		Short:   "Undelete a retainer",
		Service: "Retainers", Method: "Undelete",
		Keys:  []string{"Invoices/Retainers/Undelete Retainer"},
		Class: ClassI, Scope: ScopeBusiness, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Retainers.Undelete(ctx, inv.Scope.BusinessID, inv.IntID())
		},
	},
}
