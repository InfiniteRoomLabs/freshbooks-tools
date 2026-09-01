package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type callbacksRegisterIn struct {
	AcctScope
	Body freshbooks.CallbackRegisterRequest `json:"body" jsonschema:"the webhook callback fields to register"`
}

type callbacksListIn struct {
	AcctScope
	listIn
}

type callbacksIDIn struct {
	AcctScope
	ID int64 `json:"id" jsonschema:"the callback id"`
}

type callbacksVerifyIn struct {
	AcctScope
	ID       int64  `json:"id" jsonschema:"the callback id"`
	Verifier string `json:"verifier" jsonschema:"the verification code FreshBooks sent to the callback URI"`
}

// callbacksSpecs are the tools wrapping *freshbooks.CallbacksService.
var callbacksSpecs = []Spec{
	newSpec("callbacks_register",
		"Register a webhook callback URI for a business's events. See https://www.freshbooks.com/api/webhooks.",
		"Callbacks", "Register",
		[]string{"Webhooks/Register for Callback"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in callbacksRegisterIn) (any, error) {
			return c.Callbacks.Register(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("callbacks_list",
		"List a business's registered webhook callbacks. See https://www.freshbooks.com/api/webhooks.",
		"Callbacks", "List",
		[]string{"Webhooks/List Webhook Callbacks"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in callbacksListIn) (any, error) {
			return c.Callbacks.List(ctx, scope.AccountID, &freshbooks.CallbackListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("callbacks_delete",
		"Delete a webhook callback. See https://www.freshbooks.com/api/webhooks.",
		"Callbacks", "Delete",
		[]string{"Webhooks/Delete Webhook Callback"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in callbacksIDIn) (any, error) {
			return void(c.Callbacks.Delete(ctx, scope.AccountID, in.ID))
		}),
	newSpec("callbacks_verify",
		"Verify a webhook callback with the code FreshBooks sent to it. See https://www.freshbooks.com/api/webhooks.",
		"Callbacks", "Verify",
		[]string{"Webhooks/Verify Webhook Callback"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in callbacksVerifyIn) (any, error) {
			return c.Callbacks.Verify(ctx, scope.AccountID, in.ID, in.Verifier)
		}),
	newSpec("callbacks_resend_verification",
		"Resend a webhook callback's verification code. See https://www.freshbooks.com/api/webhooks.",
		"Callbacks", "ResendVerification",
		[]string{"Webhooks/Resend Verification Code"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in callbacksIDIn) (any, error) {
			return void(c.Callbacks.ResendVerification(ctx, scope.AccountID, in.ID))
		}),
}
