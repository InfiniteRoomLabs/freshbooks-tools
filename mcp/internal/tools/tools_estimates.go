package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type estimatesListIn struct {
	AcctScope
	listIn
	includeIn
}

type estimatesGetIn struct {
	AcctScope
	idIn
	includeIn
}

type estimatesCreateIn struct {
	AcctScope
	Body freshbooks.EstimateWriteRequest `json:"body" jsonschema:"the estimate fields to create"`
}

type estimatesUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.EstimateWriteRequest `json:"body" jsonschema:"the estimate fields to update"`
}

type estimatesIDIn struct {
	AcctScope
	idIn
}

type estimatesSendIn struct {
	AcctScope
	idIn
	Body freshbooks.EstimateSendRequest `json:"body" jsonschema:"the email fields to send the estimate with"`
}

// estimatesSpecs are the tools wrapping *freshbooks.EstimatesService.
var estimatesSpecs = []Spec{
	newSpec("estimates_list",
		"List an account's estimates. See https://www.freshbooks.com/api/estimates.",
		"Estimates", "List",
		[]string{"Estimates/List Estimates"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in estimatesListIn) (any, error) {
			return c.Estimates.List(ctx, scope.AccountID, &freshbooks.EstimateListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage, Include: in.Include})
		}),
	newSpec("estimates_get",
		"Get a single estimate. See https://www.freshbooks.com/api/estimates.",
		"Estimates", "Get",
		[]string{"Estimates/Single Estimate"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in estimatesGetIn) (any, error) {
			return c.Estimates.Get(ctx, scope.AccountID, in.ID, in.opts()...)
		}),
	newSpec("estimates_create",
		"Create an estimate or proposal, optionally with sections, a logo, and e-signature. See https://www.freshbooks.com/api/estimates.",
		"Estimates", "Create",
		[]string{"Estimates/Create Single Proposal w/ Sections, Logos, and E-signature", "Estimates/Single Estimate With Estimate Lines"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in estimatesCreateIn) (any, error) {
			return c.Estimates.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("estimates_update",
		"Update an estimate. See https://www.freshbooks.com/api/estimates.",
		"Estimates", "Update",
		[]string{"Estimates/Update Estimate"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in estimatesUpdateIn) (any, error) {
			return c.Estimates.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("estimates_delete",
		"Delete an estimate. See https://www.freshbooks.com/api/estimates.",
		"Estimates", "Delete",
		[]string{"Estimates/Delete Estimate"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in estimatesIDIn) (any, error) {
			return void(c.Estimates.Delete(ctx, scope.AccountID, in.ID))
		}),
	newSpec("estimates_accept",
		"Mark an estimate as accepted by the client. See https://www.freshbooks.com/api/estimates.",
		"Estimates", "Accept",
		[]string{"Estimates/Accept Estimate"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in estimatesIDIn) (any, error) {
			return c.Estimates.Accept(ctx, scope.AccountID, in.ID)
		}),
	newSpec("estimates_send",
		"Send an estimate to the client by email. See https://www.freshbooks.com/api/estimates.",
		"Estimates", "Send",
		[]string{"Estimates/Send Estimate by Email"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in estimatesSendIn) (any, error) {
			return void(c.Estimates.Send(ctx, scope.AccountID, in.ID, &in.Body))
		}),
}
