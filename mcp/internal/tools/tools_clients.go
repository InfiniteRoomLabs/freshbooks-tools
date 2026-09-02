package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type clientsListIn struct {
	AcctScope
	listIn
	includeIn
}

type clientsGetIn struct {
	AcctScope
	idIn
	includeIn
}

type clientsCreateIn struct {
	AcctScope
	Body freshbooks.ClientWriteRequest `json:"body" jsonschema:"the client fields to create"`
}

type clientsUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.ClientWriteRequest `json:"body" jsonschema:"the client fields to update"`
}

type clientsIDIn struct {
	AcctScope
	idIn
}

// clientsSpecs are the tools wrapping *freshbooks.ClientsService.
var clientsSpecs = []Spec{
	newSpec("clients_list",
		"List an account's clients. See https://www.freshbooks.com/api/clients.",
		"Clients", "List",
		[]string{"Clients/List Clients"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in clientsListIn) (any, error) {
			return c.Clients.List(ctx, scope.AccountID, &freshbooks.ClientListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage, Include: in.Include})
		}),
	newSpec("clients_get",
		"Get a single client. See https://www.freshbooks.com/api/clients.",
		"Clients", "Get",
		[]string{"Clients/Single Client"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in clientsGetIn) (any, error) {
			return c.Clients.Get(ctx, scope.AccountID, in.ID, in.opts()...)
		}),
	newSpec("clients_create",
		"Create a new client. See https://www.freshbooks.com/api/clients.",
		"Clients", "Create",
		[]string{"Clients/New Client"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in clientsCreateIn) (any, error) {
			return c.Clients.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("clients_update",
		"Update a client. See https://www.freshbooks.com/api/clients.",
		"Clients", "Update",
		[]string{"Clients/Update Client"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in clientsUpdateIn) (any, error) {
			return c.Clients.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("clients_remove_all_secondary_contacts",
		"Remove every secondary contact from a client. See https://www.freshbooks.com/api/clients.",
		"Clients", "RemoveAllSecondaryContacts",
		[]string{"Clients/Remove All Secondary Contacts"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in clientsIDIn) (any, error) {
			return c.Clients.RemoveAllSecondaryContacts(ctx, scope.AccountID, in.ID)
		}),
}
