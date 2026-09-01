package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type servicesGetIn struct {
	BizScope
	ServiceID int64 `json:"service_id" jsonschema:"the service id"`
}

type servicesListIn struct {
	BizScope
	listIn
}

// servicesCreateIn is account-scoped, not business-scoped: Services.Create
// (see freshbooks/services_svc.go) takes an AccountID where every other
// method on ServicesService takes a BusinessID -- confirmed against the
// lib signature, not an inconsistency to "fix" here.
type servicesCreateIn struct {
	AcctScope
	Body freshbooks.BillableItemCreateRequest `json:"body" jsonschema:"the billable-item fields to create"`
}

type servicesGetBillableItemIn struct {
	AcctScope
	idIn
}

// servicesSpecs are the tools wrapping *freshbooks.ServicesService.
var servicesSpecs = []Spec{
	newSpec("services_get",
		"Get a single service. See https://www.freshbooks.com/api/projects.",
		"Services", "Get",
		[]string{"Settings/Items and Services/Get a Single Service"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in servicesGetIn) (any, error) {
			return c.Services.Get(ctx, scope.BusinessID, in.ServiceID)
		}),
	newSpec("services_list",
		"List a business's services. See https://www.freshbooks.com/api/projects.",
		"Services", "List",
		[]string{"Settings/Items and Services/List Services"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in servicesListIn) (any, error) {
			return c.Services.List(ctx, scope.BusinessID, &freshbooks.ServiceListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("services_create",
		"Create a billable service. See https://www.freshbooks.com/api/projects.",
		"Services", "Create",
		[]string{"Settings/Items and Services/Create Service"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in servicesCreateIn) (any, error) {
			return c.Services.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("services_get_billable_item",
		"Get a single billable item. See https://www.freshbooks.com/api/projects.",
		"Services", "GetBillableItem",
		[]string{"Settings/Items and Services/Single Service"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in servicesGetBillableItemIn) (any, error) {
			return c.Services.GetBillableItem(ctx, scope.AccountID, in.ID)
		}),
}
