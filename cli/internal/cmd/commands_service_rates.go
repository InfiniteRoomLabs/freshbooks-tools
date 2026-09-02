package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/pflag"
)

// serviceRatesCommands wrap *freshbooks.ServiceRatesService.
//
// service-rates list takes no options at all (List(ctx, businessID)),
// unlike docs/phases/4/commands.md's "S, L"; the lib signature wins.
// update-project-rate needs both a project id and a service id plus a
// rate, not the "--rate/--payment-method" commands.md names (a
// copy-paste artifact shared with two other rows -- see the implementer
// report): the positional <id> is the service id, --project-id and
// --rate are flags.
var serviceRatesCommands = []Command{
	{
		Group: "service-rates", Verb: "get",
		Short:   "Get a single service rate",
		Service: "ServiceRates", Method: "Get",
		Keys:  []string{"Settings/Items and Services/Get a Single Service Rate"},
		Class: ClassRO, Scope: ScopeBusiness, HasID: true, IDName: "service-id",
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.ServiceRates.Get(ctx, inv.Scope.BusinessID, inv.IntID())
		},
	},
	{
		Group: "service-rates", Verb: "list",
		Short:   "List service rates",
		Service: "ServiceRates", Method: "List",
		Keys:  []string{"Projects/Service Rates"},
		Class: ClassRO, Scope: ScopeBusiness,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.ServiceRates.List(ctx, inv.Scope.BusinessID)
		},
	},
	{
		Group: "service-rates", Verb: "update-project-rate",
		Short:   "Update a project's rate for a service",
		Service: "ServiceRates", Method: "UpdateProjectRate",
		Keys:  []string{"Projects/Update Service Rates"},
		Class: ClassI, Scope: ScopeBusiness, HasID: true, IDName: "service-id",
		ExtraFlags: func(fs *pflag.FlagSet) {
			fs.Int64("project-id", 0, "the project id (required)")
			fs.String("rate", "", "the rate to set (required)")
		},
		RequiredFlags: []string{"rate"}, RequiredInt64Flags: []string{"project-id"},
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			projectID, _ := inv.Flags.GetInt64("project-id")
			return c.ServiceRates.UpdateProjectRate(ctx, inv.Scope.BusinessID, projectID, inv.IntID(), inv.RequiredString("rate"))
		},
	},
}
