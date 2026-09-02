package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// systemsCommands wrap *freshbooks.SystemsService.
//
// systems get needs both an account id and a business id
// (SystemsService.Get(ctx, accountID, businessID)) -- there is no
// separate resource id at all, unlike docs/phases/4/commands.md's "S,
// ID" row. ScopeAccountAndBusiness resolves both --account and
// --business; the lib signature wins.
//
// > STATE AS OF 2026-09-01 (Phase 4, CLI): spec section 7 should note
// > this command takes no positional id -- both account and business
// > identify the resource. Flagged for whoever next revises section 7.
var systemsCommands = []Command{
	{
		Group: "systems", Verb: "get",
		Short:   "Get a business's system-level settings",
		Service: "Systems", Method: "Get",
		Keys:  []string{"Settings/Systems/Get System"},
		Class: ClassRO, Scope: ScopeAccountAndBusiness,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Systems.Get(ctx, inv.Scope.AccountID, inv.Scope.BusinessID)
		},
	},
}
