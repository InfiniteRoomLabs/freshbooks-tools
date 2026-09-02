package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// creditNotesCommands wrap *freshbooks.CreditNotesService.
var creditNotesCommands = []Command{
	{
		Group: "credit-notes", Verb: "list",
		Short:   "List a client's credits",
		Service: "CreditNotes", Method: "List",
		Keys:  []string{"Clients/Credits/List Credits"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.CreditNoteListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.CreditNotes.All(ctx, inv.Scope.AccountID, opts))
			}
			return c.CreditNotes.List(ctx, inv.Scope.AccountID, opts)
		},
	},
	{
		Group: "credit-notes", Verb: "create",
		Short:   "Create a credit note or prepayment credit",
		Service: "CreditNotes", Method: "Create",
		Keys:  []string{"Clients/Credits/Create Credit Note", "Clients/Credits/Create Prepayment Credit"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.CreditNoteWriteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.CreditNotes.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "credit-notes", Verb: "update",
		Short:   "Update a credit note or prepayment credit",
		Service: "CreditNotes", Method: "Update",
		Keys:  []string{"Clients/Credits/Update Credit Note", "Clients/Credits/Update Prepayment Credit"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.CreditNoteWriteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.CreditNotes.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "credit-notes", Verb: "delete",
		Short:   "Delete a credit",
		Service: "CreditNotes", Method: "Delete",
		Keys:  []string{"Clients/Credits/Delete Credit"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.CreditNotes.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
