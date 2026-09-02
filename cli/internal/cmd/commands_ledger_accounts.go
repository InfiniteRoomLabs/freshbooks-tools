package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// ledgerAccountsCommands wrap *freshbooks.LedgerAccountsService.
//
// docs/phases/4/commands.md lists ledger-accounts list with L (list
// flags) and types/sub-types/sub-type with S (account scope), but
// List(ctx, biz) takes no options at all and Types/SubTypes/SubType(ctx[,
// id]) take no scope parameter whatsoever -- the lib signatures win; see
// the implementer report.
var ledgerAccountsCommands = []Command{
	{
		Group: "ledger-accounts", Verb: "create",
		Short:   "Create a ledger account",
		Service: "LedgerAccounts", Method: "Create",
		Keys:  []string{"Accounting/Accounts/Create Account"},
		Class: ClassW, Scope: ScopeBusinessUUID, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.LedgerAccountCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.LedgerAccounts.Create(ctx, inv.Scope.BusinessUUID, &body)
		},
	},
	{
		Group: "ledger-accounts", Verb: "list",
		Short:   "List ledger accounts",
		Service: "LedgerAccounts", Method: "List",
		Keys:  []string{"Accounting/Accounts/List Accounts"},
		Class: ClassRO, Scope: ScopeBusinessUUID,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.LedgerAccounts.List(ctx, inv.Scope.BusinessUUID)
		},
	},
	{
		Group: "ledger-accounts", Verb: "get",
		Short:   "Get a single ledger account",
		Service: "LedgerAccounts", Method: "Get",
		Keys:  []string{"Accounting/Accounts/Single Account"},
		Class: ClassRO, Scope: ScopeBusinessUUID, HasID: true, IDKind: "string", IDName: "account-uuid",
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.LedgerAccounts.Get(ctx, inv.Scope.BusinessUUID, inv.StrID())
		},
	},
	{
		Group: "ledger-accounts", Verb: "update",
		Short:   "Update a ledger account",
		Service: "LedgerAccounts", Method: "Update",
		Keys:  []string{"Accounting/Accounts/Update Account"},
		Class: ClassI, Scope: ScopeBusinessUUID, HasID: true, IDKind: "string", IDName: "account-uuid", Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.LedgerAccountUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.LedgerAccounts.Update(ctx, inv.Scope.BusinessUUID, inv.StrID(), &body)
		},
	},
	{
		Group: "ledger-accounts", Verb: "types",
		Short:   "List ledger account types",
		Service: "LedgerAccounts", Method: "Types",
		Keys:  []string{"Accounting/Accounts/List Account types"},
		Class: ClassRO,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.LedgerAccounts.Types(ctx)
		},
	},
	{
		Group: "ledger-accounts", Verb: "sub-types",
		Short:   "List ledger account sub-types",
		Service: "LedgerAccounts", Method: "SubTypes",
		Keys:  []string{"Accounting/Accounts/List Sub types"},
		Class: ClassRO,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.LedgerAccounts.SubTypes(ctx)
		},
	},
	{
		Group: "ledger-accounts", Verb: "sub-type",
		Short:   "Get a single ledger account sub-type",
		Service: "LedgerAccounts", Method: "SubType",
		Keys:  []string{"Accounting/Accounts/Single Sub type"},
		Class: ClassRO, HasID: true, IDKind: "string",
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.LedgerAccounts.SubType(ctx, inv.StrID())
		},
	},
}
