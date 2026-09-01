package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type identityRegisterIn struct {
	Body freshbooks.RegisterRequest `json:"body" jsonschema:"the new user's registration fields"`
}

type identityAddBusinessIn struct {
	Body freshbooks.BusinessCreateRequest `json:"body" jsonschema:"the business fields to create"`
}

type identityProvisionPaymentsIn struct {
	AcctScope
	Body freshbooks.PaymentsProvisionRequest `json:"body" jsonschema:"the FreshBooks Payments enrollment fields"`
}

type identityCreateApplicationIn struct {
	Body freshbooks.ApplicationCreateRequest `json:"body" jsonschema:"the OAuth application fields to register"`
}

// identityUpdateApplicationIn's Body.ClientSecret is a required input
// string (freshbooks/settings.go's ApplicationUpdateRequest has no
// omitempty on it), so the tool is registered with newSensitiveSpec, not
// newSpec: see that function's doc comment and
// docs/phases/3/reports/security.md finding 1 for why a required secret
// input needs it.
type identityUpdateApplicationIn struct {
	ClientID string                              `json:"client_id" jsonschema:"the OAuth application's client id"`
	Body     freshbooks.ApplicationUpdateRequest `json:"body" jsonschema:"the OAuth application's full replacement fields"`
}

// redactApplication zeroes ClientSecret before an Application reaches a
// tool result. ClientSecret is omitempty, so zeroing it removes the field
// from the wire entirely rather than emitting an empty string -- a
// stateless, model-facing tool result is not the place for a live OAuth
// credential (see freshbooks/settings.go's Application doc comment and
// docs/phases/3/plan.md's written security constraints).
func redactApplication(a *freshbooks.Application) *freshbooks.Application {
	if a != nil {
		a.ClientSecret = ""
	}
	return a
}

func redactApplications(apps []freshbooks.Application) []freshbooks.Application {
	for i := range apps {
		apps[i].ClientSecret = ""
	}
	return apps
}

// identitySpecs are the tools wrapping *freshbooks.IdentityService, both
// its identity.go methods and the business/application-management methods
// settings.go adds to the same service.
var identitySpecs = []Spec{
	newSpec("identity_me",
		"List the businesses (memberships) the current token's identity belongs to. See https://www.freshbooks.com/api/authentication.",
		"Identity", "Me",
		[]string{"Authorization/Identity Info Call", "Authorization/List User"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in emptyIn) (any, error) {
			return c.Identity.Me(ctx)
		}),
	newSpec("identity_whoami",
		"Get the full identity behind the current token, memberships included. Always available for a client to discover its account_id/business_id/business_uuid. See https://www.freshbooks.com/api/authentication.",
		"Identity", "Whoami",
		nil, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in emptyIn) (any, error) {
			return c.Identity.Whoami(ctx)
		}),
	newSpec("identity_register",
		"Register a brand-new FreshBooks user. See https://www.freshbooks.com/api/authentication.",
		"Identity", "Register",
		[]string{"Authorization/Register as a new user"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in identityRegisterIn) (any, error) {
			return c.Identity.Register(ctx, &in.Body)
		}),
	newSpec("identity_add_business",
		"Provision a new business under the current identity. See https://www.freshbooks.com/api/authentication.",
		"Identity", "AddBusiness",
		[]string{"Settings/Businesses/Add Business"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in identityAddBusinessIn) (any, error) {
			return c.Identity.AddBusiness(ctx, &in.Body)
		}),
	newSpec("identity_delete_business",
		"Delete a business. Destructive and irreversible: FreshBooks refuses this while the business carries an active subscription -- cancel it first with identity_delete_business_subscription. See https://www.freshbooks.com/api/authentication.",
		"Identity", "DeleteBusiness",
		[]string{"Settings/Businesses/Delete Business"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in BizScope) (any, error) {
			return void(c.Identity.DeleteBusiness(ctx, scope.BusinessID))
		}),
	newSpec("identity_delete_business_subscription",
		"Cancel a business's billing subscription, the prerequisite for identity_delete_business. Destructive and irreversible. See https://www.freshbooks.com/api/authentication.",
		"Identity", "DeleteBusinessSubscription",
		[]string{"Settings/Businesses/Delete Business - Subscription"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in AcctScope) (any, error) {
			return void(c.Identity.DeleteBusinessSubscription(ctx, scope.AccountID))
		}),
	newSpec("identity_provision_payments",
		"Enroll a business in FreshBooks Payments. See https://www.freshbooks.com/api/authentication.",
		"Identity", "ProvisionPayments",
		[]string{"Settings/Businesses/Provision FreshBooks Payments"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in identityProvisionPaymentsIn) (any, error) {
			return void(c.Identity.ProvisionPayments(ctx, scope.AccountID, &in.Body))
		}),
	newSpec("identity_create_application",
		"Register a new OAuth application. The response's client_secret is redacted before it reaches this tool's result. See https://www.freshbooks.com/api/authentication.",
		"Identity", "CreateApplication",
		[]string{"Settings/Developer/Create new application"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in identityCreateApplicationIn) (any, error) {
			app, err := c.Identity.CreateApplication(ctx, &in.Body)
			if err != nil {
				return nil, err
			}
			return redactApplication(app), nil
		}),
	newSpec("identity_applications",
		"List the current identity's registered OAuth applications. Each result's client_secret is redacted. See https://www.freshbooks.com/api/authentication.",
		"Identity", "Applications",
		[]string{"Settings/Developer/Get all applications"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in emptyIn) (any, error) {
			apps, err := c.Identity.Applications(ctx)
			if err != nil {
				return nil, err
			}
			return redactApplications(apps), nil
		}),
	newSensitiveSpec("identity_update_application",
		"Edit a registered OAuth application. The response's client_secret is redacted before it reaches this tool's result. See https://www.freshbooks.com/api/authentication.",
		"Identity", "UpdateApplication",
		[]string{"Settings/Developer/Modify existing application"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in identityUpdateApplicationIn) (any, error) {
			app, err := c.Identity.UpdateApplication(ctx, in.ClientID, &in.Body)
			if err != nil {
				return nil, err
			}
			return redactApplication(app), nil
		}),
}
