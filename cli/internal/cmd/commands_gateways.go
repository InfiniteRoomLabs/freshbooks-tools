package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// gatewaysCommands wrap *freshbooks.GatewaysService.
//
// docs/phases/4/commands.md lists gateways get as taking a positional
// <id>, but GatewaysService.Get(ctx, acct) takes no id at all -- it
// returns every configured gateway connection for the account. The lib
// method signature wins (docs/phases/4/plan.md's stated tie-break rule);
// see the implementer report for the full discrepancy list.
var gatewaysCommands = []Command{
	{
		Group: "gateways", Verb: "get",
		Short:   "List an account's configured payment gateway connections",
		Service: "Gateways", Method: "Get",
		Keys:  []string{"Tokenization/1a. [STRIPE] -  Get Publishable Key", "Settings/Businesses/Gateway Details", "Settings/Gateways/List Gateways"},
		Class: ClassRO, Scope: ScopeAccount,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Gateways.Get(ctx, inv.Scope.AccountID)
		},
	},
}
