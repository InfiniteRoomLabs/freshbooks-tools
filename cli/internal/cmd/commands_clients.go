package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// clientsCommands wrap *freshbooks.ClientsService.
var clientsCommands = []Command{
	{
		Group: "clients", Verb: "list",
		Short:   "List an account's clients",
		Service: "Clients", Method: "List",
		Keys:  []string{"Clients/List Clients"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasInclude: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.ClientListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage(), Include: inv.Include()}
			if inv.All() {
				return collectAll(c.Clients.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.Clients.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "clients", Verb: "get",
		Short:   "Get a single client",
		Service: "Clients", Method: "Get",
		Keys:  []string{"Clients/Single Client"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true, HasInclude: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Clients.Get(ctx, inv.Scope.AccountID, inv.IntID(), inv.IncludeOpt()...)
		},
	},
	{
		Group: "clients", Verb: "create",
		Short:   "Create a new client",
		Service: "Clients", Method: "Create",
		Keys:  []string{"Clients/New Client"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ClientWriteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Clients.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "clients", Verb: "update",
		Short:   "Update a client",
		Service: "Clients", Method: "Update",
		Keys:  []string{"Clients/Update Client"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ClientWriteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Clients.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "clients", Verb: "remove-all-secondary-contacts",
		Short:   "Remove every secondary contact from a client",
		Service: "Clients", Method: "RemoveAllSecondaryContacts",
		Keys:  []string{"Clients/Remove All Secondary Contacts"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Clients.RemoveAllSecondaryContacts(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
}
