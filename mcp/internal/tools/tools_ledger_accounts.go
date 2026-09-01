package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type ledgerAccountsCreateIn struct {
	UUIDScope
	Body freshbooks.LedgerAccountCreateRequest `json:"body" jsonschema:"the ledger account fields to create"`
}

type ledgerAccountsGetIn struct {
	UUIDScope
	AccountUUID string `json:"account_uuid" jsonschema:"the ledger account's UUID"`
}

type ledgerAccountsUpdateIn struct {
	UUIDScope
	AccountUUID string                                `json:"account_uuid" jsonschema:"the ledger account's UUID"`
	Body        freshbooks.LedgerAccountUpdateRequest `json:"body" jsonschema:"the ledger account fields to update"`
}

type ledgerAccountsSubTypeIn struct {
	ID string `json:"id" jsonschema:"the ledger account sub-type id"`
}

// ledgerAccountsSpecs are the tools wrapping
// *freshbooks.LedgerAccountsService. Types, SubTypes, and SubType are
// global taxonomy lookups: the lib methods take no business scope at all.
var ledgerAccountsSpecs = []Spec{
	newSpec("ledger_accounts_create",
		"Create a chart-of-accounts ledger account. See https://www.freshbooks.com/api/accounting.",
		"LedgerAccounts", "Create",
		[]string{"Accounting/Accounts/Create Account"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in ledgerAccountsCreateIn) (any, error) {
			return c.LedgerAccounts.Create(ctx, scope.BusinessUUID, &in.Body)
		}),
	newSpec("ledger_accounts_list",
		"List a business's chart-of-accounts ledger accounts. See https://www.freshbooks.com/api/accounting.",
		"LedgerAccounts", "List",
		[]string{"Accounting/Accounts/List Accounts"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in UUIDScope) (any, error) {
			return c.LedgerAccounts.List(ctx, scope.BusinessUUID)
		}),
	newSpec("ledger_accounts_get",
		"Get a single ledger account. See https://www.freshbooks.com/api/accounting.",
		"LedgerAccounts", "Get",
		[]string{"Accounting/Accounts/Single Account"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in ledgerAccountsGetIn) (any, error) {
			return c.LedgerAccounts.Get(ctx, scope.BusinessUUID, in.AccountUUID)
		}),
	newSpec("ledger_accounts_update",
		"Update a ledger account. See https://www.freshbooks.com/api/accounting.",
		"LedgerAccounts", "Update",
		[]string{"Accounting/Accounts/Update Account"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in ledgerAccountsUpdateIn) (any, error) {
			return c.LedgerAccounts.Update(ctx, scope.BusinessUUID, in.AccountUUID, &in.Body)
		}),
	newSpec("ledger_accounts_types",
		"List the ledger account types the chart of accounts supports. See https://www.freshbooks.com/api/accounting.",
		"LedgerAccounts", "Types",
		[]string{"Accounting/Accounts/List Account types"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in emptyIn) (any, error) {
			return c.LedgerAccounts.Types(ctx)
		}),
	newSpec("ledger_accounts_sub_types",
		"List the ledger account sub-types the chart of accounts supports. See https://www.freshbooks.com/api/accounting.",
		"LedgerAccounts", "SubTypes",
		[]string{"Accounting/Accounts/List Sub types"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in emptyIn) (any, error) {
			return c.LedgerAccounts.SubTypes(ctx)
		}),
	newSpec("ledger_accounts_sub_type",
		"Get a single ledger account sub-type. See https://www.freshbooks.com/api/accounting.",
		"LedgerAccounts", "SubType",
		[]string{"Accounting/Accounts/Single Sub type"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in ledgerAccountsSubTypeIn) (any, error) {
			return c.LedgerAccounts.SubType(ctx, in.ID)
		}),
}
