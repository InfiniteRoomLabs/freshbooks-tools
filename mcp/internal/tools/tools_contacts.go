package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type contactsUpdateIn struct {
	AcctScope
	ID   int64                           `json:"id" jsonschema:"the secondary contact id"`
	Body freshbooks.ContactUpdateRequest `json:"body" jsonschema:"the secondary contact fields to update"`
}

type contactsIDIn struct {
	AcctScope
	ID int64 `json:"id" jsonschema:"the secondary contact id"`
}

// contactsSpecs are the tools wrapping *freshbooks.ContactsService.
var contactsSpecs = []Spec{
	newSpec("contacts_update",
		"Update a client's secondary contact. See https://www.freshbooks.com/api/clients.",
		"Contacts", "Update",
		[]string{"Clients/Edit Secondary Contact ID"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in contactsUpdateIn) (any, error) {
			return c.Contacts.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("contacts_delete",
		"Delete a client's secondary contact. See https://www.freshbooks.com/api/clients.",
		"Contacts", "Delete",
		// The tools.md source key carries a double space between "Secondary"
		// and "Contact"; it is copied verbatim from the Postman collection's
		// request name (see freshbooks/internal/inventory's Normalize),
		// which the inventory tool does not trim internal whitespace from.
		[]string{"Clients/Delete Secondary  Contact ID"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in contactsIDIn) (any, error) {
			return void(c.Contacts.Delete(ctx, scope.AccountID, in.ID))
		}),
}
