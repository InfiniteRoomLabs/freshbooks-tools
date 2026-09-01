package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type systemsGetIn struct {
	AcctScope
	BizScope
}

// systemsSpecs are the tools wrapping *freshbooks.SystemsService.
var systemsSpecs = []Spec{
	newSpec("systems_get",
		"Get an account's system settings for a business. See https://www.freshbooks.com/api/other_apis.",
		"Systems", "Get",
		[]string{"Settings/Systems/Get System"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in systemsGetIn) (any, error) {
			return c.Systems.Get(ctx, scope.AccountID, scope.BusinessID)
		}),
}
