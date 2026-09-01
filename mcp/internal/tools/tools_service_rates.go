package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type serviceRatesGetIn struct {
	BizScope
	ServiceID int64 `json:"service_id" jsonschema:"the service id"`
}

type serviceRatesUpdateProjectRateIn struct {
	BizScope
	ProjectID int64  `json:"project_id" jsonschema:"the project id"`
	ServiceID int64  `json:"service_id" jsonschema:"the service id"`
	Rate      string `json:"rate" jsonschema:"the new hourly rate, as a decimal string"`
}

// serviceRatesSpecs are the tools wrapping *freshbooks.ServiceRatesService.
var serviceRatesSpecs = []Spec{
	newSpec("service_rates_get",
		"Get a single service's rate. See https://www.freshbooks.com/api/projects.",
		"ServiceRates", "Get",
		[]string{"Settings/Items and Services/Get a Single Service Rate"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in serviceRatesGetIn) (any, error) {
			return c.ServiceRates.Get(ctx, scope.BusinessID, in.ServiceID)
		}),
	newSpec("service_rates_list",
		"List a business's service rates. See https://www.freshbooks.com/api/projects.",
		"ServiceRates", "List",
		[]string{"Projects/Service Rates"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in BizScope) (any, error) {
			return c.ServiceRates.List(ctx, scope.BusinessID)
		}),
	newSpec("service_rates_update_project_rate",
		"Update a service's rate on a specific project. See https://www.freshbooks.com/api/projects.",
		"ServiceRates", "UpdateProjectRate",
		[]string{"Projects/Update Service Rates"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in serviceRatesUpdateProjectRateIn) (any, error) {
			return c.ServiceRates.UpdateProjectRate(ctx, scope.BusinessID, in.ProjectID, in.ServiceID, in.Rate)
		}),
}
