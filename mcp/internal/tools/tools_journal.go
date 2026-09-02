package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type journalEntriesCreateIn struct {
	AcctScope
	Body freshbooks.JournalEntryCreateRequest `json:"body" jsonschema:"the journal entry fields to create"`
}

type journalEntriesDetailsIn struct {
	AcctScope
	listIn
}

type journalEntryAccountsListIn struct {
	AcctScope
	listIn
}

// journalSpecs are the tools wrapping *freshbooks.JournalEntriesService and
// *freshbooks.JournalEntryAccountsService.
var journalSpecs = []Spec{
	newSpec("journal_entries_create",
		"Add a journal entry. See https://www.freshbooks.com/api/accounting.",
		"JournalEntries", "Create",
		[]string{"Accounting/Journal Entries/Add Journal Entry"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in journalEntriesCreateIn) (any, error) {
			return c.JournalEntries.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("journal_entries_details",
		"List an account's journal entry details. See https://www.freshbooks.com/api/accounting.",
		"JournalEntries", "Details",
		[]string{"Accounting/Journal Entries/Journal Entry Details"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in journalEntriesDetailsIn) (any, error) {
			return c.JournalEntries.Details(ctx, scope.AccountID, in.reqOpts()...)
		}),
	newSpec("journal_entry_accounts_list",
		"List an account's journal entry (ledger) accounts. See https://www.freshbooks.com/api/accounting.",
		"JournalEntryAccounts", "List",
		[]string{"Accounting/Journal Entries/Accounts", "Reports/General Ledger"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in journalEntryAccountsListIn) (any, error) {
			return c.JournalEntryAccounts.List(ctx, scope.AccountID, in.reqOpts()...)
		}),
}
