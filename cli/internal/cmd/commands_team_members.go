package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/pflag"
)

// teamMembersCommands wrap *freshbooks.TeamMembersService.
//
// team-members invite's lib method (Invite(ctx, req)) takes no scope
// parameter at all -- the business is presumably carried inside
// InviteRequest itself -- unlike docs/phases/4/commands.md's "S, B"; the
// lib signature wins. update-rate's "--rate/--payment-method" column is
// the same copy-paste artifact noted on service-rates update-project-rate;
// the lib method takes only a rate, no payment method.
var teamMembersCommands = []Command{
	{
		Group: "team-members", Verb: "list",
		Short:   "List team members",
		Service: "TeamMembers", Method: "List",
		Keys:  []string{"My Team/List Team Members"},
		Class: ClassRO, Scope: ScopeBusiness, List: true, HasAll: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.TeamMemberListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.TeamMembers.All(ctx, inv.Scope.BusinessID, opts))
			}
			return c.TeamMembers.List(ctx, inv.Scope.BusinessID, opts)
		},
	},
	{
		Group: "team-members", Verb: "get",
		Short:   "Get a single team member",
		Service: "TeamMembers", Method: "Get",
		Keys:  []string{"My Team/Single Team Member"},
		Class: ClassRO, Scope: ScopeBusiness, HasID: true, IDKind: "string", IDName: "team-member-uuid",
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.TeamMembers.Get(ctx, inv.Scope.BusinessID, inv.StrID())
		},
	},
	{
		Group: "team-members", Verb: "invitation-rates",
		Short:   "List a business's invitation rates",
		Service: "TeamMembers", Method: "InvitationRates",
		Keys:  []string{"Projects/Invitation Rates"},
		Class: ClassRO, Scope: ScopeBusiness,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.TeamMembers.InvitationRates(ctx, inv.Scope.BusinessID)
		},
	},
	{
		Group: "team-members", Verb: "rates",
		Short:   "List a business's team member rates",
		Service: "TeamMembers", Method: "Rates",
		Keys:  []string{"Projects/Team Member Rates"},
		Class: ClassRO, Scope: ScopeBusiness,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.TeamMembers.Rates(ctx, inv.Scope.BusinessID)
		},
	},
	{
		Group: "team-members", Verb: "update-rate",
		Short:   "Update a team member's rate",
		Service: "TeamMembers", Method: "UpdateRate",
		Keys:  []string{"My Team/Update Staff Rates", "Projects/Update Team Member Rate"},
		Class: ClassI, Scope: ScopeBusiness, HasID: true, IDName: "identity-id",
		ExtraFlags: func(fs *pflag.FlagSet) {
			fs.String("rate", "", "the rate to set (required)")
		},
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			rate, _ := inv.Flags.GetString("rate")
			if rate == "" {
				return nil, newUsageError("--rate is required")
			}
			return c.TeamMembers.UpdateRate(ctx, inv.Scope.BusinessID, inv.IntID(), rate)
		},
	},
	{
		Group: "team-members", Verb: "invite",
		Short:   "Invite a team member to one or more projects",
		Service: "TeamMembers", Method: "Invite",
		Keys:  []string{"Projects/Invite Team Member to Project(s)"},
		Class: ClassW, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.InviteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.TeamMembers.Invite(ctx, &body)
		},
	},
}
