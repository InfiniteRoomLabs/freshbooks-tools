package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type creditNotesListIn struct {
	AcctScope
	listIn
}

type creditNotesCreateIn struct {
	AcctScope
	Body freshbooks.CreditNoteWriteRequest `json:"body" jsonschema:"the credit note fields to create"`
}

type creditNotesUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.CreditNoteWriteRequest `json:"body" jsonschema:"the credit note fields to update"`
}

type creditNotesIDIn struct {
	AcctScope
	idIn
}

// creditNotesSpecs are the tools wrapping *freshbooks.CreditNotesService.
var creditNotesSpecs = []Spec{
	newSpec("credit_notes_list",
		"List a client's credit notes and prepayment credits. See https://www.freshbooks.com/api/clients.",
		"CreditNotes", "List",
		[]string{"Clients/Credits/List Credits"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in creditNotesListIn) (any, error) {
			return c.CreditNotes.List(ctx, scope.AccountID, &freshbooks.CreditNoteListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("credit_notes_create",
		"Create a credit note or prepayment credit for a client. See https://www.freshbooks.com/api/clients.",
		"CreditNotes", "Create",
		[]string{"Clients/Credits/Create Credit Note", "Clients/Credits/Create Prepayment Credit"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in creditNotesCreateIn) (any, error) {
			return c.CreditNotes.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("credit_notes_update",
		"Update a credit note or prepayment credit. See https://www.freshbooks.com/api/clients.",
		"CreditNotes", "Update",
		[]string{"Clients/Credits/Update Credit Note", "Clients/Credits/Update Prepayment Credit"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in creditNotesUpdateIn) (any, error) {
			return c.CreditNotes.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("credit_notes_delete",
		"Delete a credit note. See https://www.freshbooks.com/api/clients.",
		"CreditNotes", "Delete",
		[]string{"Clients/Credits/Delete Credit"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in creditNotesIDIn) (any, error) {
			return void(c.CreditNotes.Delete(ctx, scope.AccountID, in.ID))
		}),
}
