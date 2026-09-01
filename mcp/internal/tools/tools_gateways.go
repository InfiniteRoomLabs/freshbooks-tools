package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// gatewaysSpecs are the tools wrapping *freshbooks.GatewaysService.
var gatewaysSpecs = []Spec{
	newSpec("gateways_get",
		"List an account's connected payment gateways, including the Stripe publishable key when connected. See https://www.freshbooks.com/api/payments.",
		"Gateways", "Get",
		// The source key carries a double space between "-" and "Get",
		// copied verbatim from the Postman collection's request name.
		[]string{"Tokenization/1a. [STRIPE] -  Get Publishable Key", "Settings/Businesses/Gateway Details", "Settings/Gateways/List Gateways"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in AcctScope) (any, error) {
			return c.Gateways.Get(ctx, scope.AccountID)
		}),
}
