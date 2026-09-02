package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type teamMembersListIn struct {
	BizScope
	listIn
}

type teamMembersGetIn struct {
	BizScope
	TeamMemberUUID string `json:"team_member_uuid" jsonschema:"the team member's UUID"`
}

type teamMembersUpdateRateIn struct {
	BizScope
	IdentityID int64  `json:"identity_id" jsonschema:"the team member's identity id"`
	Rate       string `json:"rate" jsonschema:"the new hourly rate, as a decimal string"`
}

type teamMembersInviteIn struct {
	Body freshbooks.InviteRequest `json:"body" jsonschema:"the team member invitation fields"`
}

// teamMembersSpecs are the tools wrapping *freshbooks.TeamMembersService.
var teamMembersSpecs = []Spec{
	newSpec("team_members_list",
		"List a business's team members. See https://www.freshbooks.com/api/other_apis.",
		"TeamMembers", "List",
		[]string{"My Team/List Team Members"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in teamMembersListIn) (any, error) {
			return c.TeamMembers.List(ctx, scope.BusinessID, &freshbooks.TeamMemberListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("team_members_get",
		"Get a single team member. See https://www.freshbooks.com/api/other_apis.",
		"TeamMembers", "Get",
		[]string{"My Team/Single Team Member"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in teamMembersGetIn) (any, error) {
			return c.TeamMembers.Get(ctx, scope.BusinessID, in.TeamMemberUUID)
		}),
	newSpec("team_members_invitation_rates",
		"List a business's team member invitation rates. See https://www.freshbooks.com/api/projects.",
		"TeamMembers", "InvitationRates",
		[]string{"Projects/Invitation Rates"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in BizScope) (any, error) {
			return c.TeamMembers.InvitationRates(ctx, scope.BusinessID)
		}),
	newSpec("team_members_rates",
		"List a business's team member rates. See https://www.freshbooks.com/api/projects.",
		"TeamMembers", "Rates",
		[]string{"Projects/Team Member Rates"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in BizScope) (any, error) {
			return c.TeamMembers.Rates(ctx, scope.BusinessID)
		}),
	newSpec("team_members_update_rate",
		"Update a team member's rate. See https://www.freshbooks.com/api/projects.",
		"TeamMembers", "UpdateRate",
		[]string{"My Team/Update Staff Rates", "Projects/Update Team Member Rate"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in teamMembersUpdateRateIn) (any, error) {
			return c.TeamMembers.UpdateRate(ctx, scope.BusinessID, in.IdentityID, in.Rate)
		}),
	newSpec("team_members_invite",
		"Invite a team member to one or more projects. See https://www.freshbooks.com/api/projects.",
		"TeamMembers", "Invite",
		[]string{"Projects/Invite Team Member to Project(s)"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in teamMembersInviteIn) (any, error) {
			return c.TeamMembers.Invite(ctx, &in.Body)
		}),
}
