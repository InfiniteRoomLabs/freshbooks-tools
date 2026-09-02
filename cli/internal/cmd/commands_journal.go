package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// journalCommands wrap *freshbooks.JournalEntriesService and
// *freshbooks.JournalEntryAccountsService, mirroring
// mcp/internal/tools/tools_journal.go's grouping of the two.
var journalCommands = []Command{
	{
		Group: "journal-entries", Verb: "create",
		Short:   "Add a journal entry",
		Service: "JournalEntries", Method: "Create",
		Keys:  []string{"Accounting/Journal Entries/Add Journal Entry"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.JournalEntryCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.JournalEntries.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "journal-entries", Verb: "details",
		Short:   "Get journal entry details",
		Service: "JournalEntries", Method: "Details",
		Keys:  []string{"Accounting/Journal Entries/Journal Entry Details"},
		Class: ClassRO, Scope: ScopeAccount,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.JournalEntries.Details(ctx, inv.Scope.AccountID)
		},
	},
	{
		Group: "journal-entry-accounts", Verb: "list",
		Short:   "List journal-entry accounts",
		Service: "JournalEntryAccounts", Method: "List",
		Keys:  []string{"Accounting/Journal Entries/Accounts", "Reports/General Ledger"},
		Class: ClassRO, Scope: ScopeAccount, List: true, NoPaging: false,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.JournalEntryAccounts.List(ctx, inv.Scope.AccountID, inv.ReqOpts()...)
		},
	},
}
