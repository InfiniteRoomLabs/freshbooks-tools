package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// retainersListIn deliberately does not embed listIn: RetainerListOptions
// carries only Search, no Page/PerPage (see freshbooks/retainers.go and
// docs/phases/3/plan.md's gotchas -- inventing pagination fields the lib
// does not support is exactly what that note warns against).
type retainersListIn struct {
	BizScope
	Search map[string]string `json:"search,omitempty" jsonschema:"filter fields as key/value pairs"`
}

type retainersIDIn struct {
	BizScope
	idIn
}

type retainersCreateIn struct {
	BizScope
	Body freshbooks.RetainerCreateRequest `json:"body" jsonschema:"the retainer fields to create"`
}

type retainersUpdateIn struct {
	BizScope
	idIn
	Body freshbooks.RetainerUpdateRequest `json:"body" jsonschema:"the retainer fields to update"`
}

// retainersSpecs are the tools wrapping *freshbooks.RetainersService.
var retainersSpecs = []Spec{
	newSpec("retainers_list",
		"List a business's retainers. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "List",
		[]string{"Invoices/Retainers/Get all retainers"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersListIn) (any, error) {
			var search freshbooks.Search
			if len(in.Search) > 0 {
				search = freshbooks.Search(in.Search)
			}
			return c.Retainers.List(ctx, scope.BusinessID, &freshbooks.RetainerListOptions{Search: search})
		}),
	newSpec("retainers_get",
		"Get a single retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Get",
		[]string{"Invoices/Retainers/Single Retainer"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersIDIn) (any, error) {
			return c.Retainers.Get(ctx, scope.BusinessID, in.ID)
		}),
	newSpec("retainers_create",
		"Create a retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Create",
		[]string{"Invoices/Retainers/Create Retainer"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersCreateIn) (any, error) {
			return c.Retainers.Create(ctx, scope.BusinessID, &in.Body)
		}),
	newSpec("retainers_update",
		"Update a retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Update",
		[]string{"Invoices/Retainers/Update Retainer"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersUpdateIn) (any, error) {
			return c.Retainers.Update(ctx, scope.BusinessID, in.ID, &in.Body)
		}),
	newSpec("retainers_delete",
		"Delete a retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Delete",
		[]string{"Invoices/Retainers/Delete Retainer"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersIDIn) (any, error) {
			return c.Retainers.Delete(ctx, scope.BusinessID, in.ID)
		}),
	newSpec("retainers_undelete",
		"Restore a deleted retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Undelete",
		[]string{"Invoices/Retainers/Undelete Retainer"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersIDIn) (any, error) {
			return c.Retainers.Undelete(ctx, scope.BusinessID, in.ID)
		}),
}
