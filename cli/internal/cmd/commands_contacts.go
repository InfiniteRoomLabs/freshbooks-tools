package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// contactsCommands wrap *freshbooks.ContactsService.
var contactsCommands = []Command{
	{
		Group: "contacts", Verb: "update",
		Short:   "Update a client's secondary contact",
		Service: "Contacts", Method: "Update",
		Keys:  []string{"Clients/Edit Secondary Contact ID"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ContactUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Contacts.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "contacts", Verb: "delete",
		Short:   "Delete a client's secondary contact",
		Service: "Contacts", Method: "Delete",
		Keys:  []string{"Clients/Delete Secondary  Contact ID"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Contacts.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
