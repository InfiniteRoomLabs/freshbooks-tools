package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// staffCommands wrap *freshbooks.StaffService, which mixes scope
// families within one service (List is business-scoped;
// Get/Update/Delete are account-scoped) -- CLAUDE.md's documented
// gotcha. staff list also takes no list options at all (List(ctx,
// businessID)), unlike docs/phases/4/commands.md's "S, L"; the lib
// signature wins.
var staffCommands = []Command{
	{
		Group: "staff", Verb: "list",
		Short:   "List a business's staff",
		Service: "Staff", Method: "List",
		Keys:  []string{"My Team/List Staff"},
		Class: ClassRO, Scope: ScopeBusiness,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Staff.List(ctx, inv.Scope.BusinessID)
		},
	},
	{
		Group: "staff", Verb: "get",
		Short:   "Get a single staff member",
		Service: "Staff", Method: "Get",
		Keys:  []string{"My Team/Single Staff"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Staff.Get(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "staff", Verb: "update",
		Short:   "Update a staff member",
		Service: "Staff", Method: "Update",
		Keys:  []string{"My Team/Update Staff"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.StaffUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Staff.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "staff", Verb: "delete",
		Short:   "Delete a staff member",
		Service: "Staff", Method: "Delete",
		Keys:  []string{"My Team/Delete Staff"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Staff.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
