package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type staffIDIn struct {
	AcctScope
	ID int64 `json:"id" jsonschema:"the staff member's id"`
}

type staffUpdateIn struct {
	AcctScope
	ID   int64                         `json:"id" jsonschema:"the staff member's id"`
	Body freshbooks.StaffUpdateRequest `json:"body" jsonschema:"the staff member fields to update"`
}

// staffSpecs are the tools wrapping *freshbooks.StaffService. List is
// business-scoped; Get/Update/Delete are account-scoped -- confirmed
// against the lib signatures, not an inconsistency to "fix" here.
var staffSpecs = []Spec{
	newSpec("staff_list",
		"List a business's staff. See https://www.freshbooks.com/api/other_apis.",
		"Staff", "List",
		[]string{"My Team/List Staff"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in BizScope) (any, error) {
			return c.Staff.List(ctx, scope.BusinessID)
		}),
	newSpec("staff_get",
		"Get a single staff member. See https://www.freshbooks.com/api/other_apis.",
		"Staff", "Get",
		[]string{"My Team/Single Staff"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in staffIDIn) (any, error) {
			return c.Staff.Get(ctx, scope.AccountID, in.ID)
		}),
	newSpec("staff_update",
		"Update a staff member. See https://www.freshbooks.com/api/other_apis.",
		"Staff", "Update",
		[]string{"My Team/Update Staff"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in staffUpdateIn) (any, error) {
			return c.Staff.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("staff_delete",
		"Delete a staff member. See https://www.freshbooks.com/api/other_apis.",
		"Staff", "Delete",
		[]string{"My Team/Delete Staff"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in staffIDIn) (any, error) {
			return void(c.Staff.Delete(ctx, scope.AccountID, in.ID))
		}),
}
