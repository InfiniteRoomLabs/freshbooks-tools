package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// TeamMember is one member of a business's team, as returned by the newer
// auth-family team-members endpoints. FreshBooks positions this as the
// replacement for the deprecated Staff resource.
type TeamMember struct {
	UUID                   string `json:"uuid"`
	FirstName              string `json:"first_name"`
	MiddleName             string `json:"middle_name"`
	LastName               string `json:"last_name"`
	Email                  string `json:"email"`
	JobTitle               string `json:"job_title"`
	Street1                string `json:"street_1"`
	Street2                string `json:"street_2"`
	City                   string `json:"city"`
	Province               string `json:"province"`
	Country                string `json:"country"`
	PostalCode             string `json:"postal_code"`
	PhoneNumber            string `json:"phone_number"`
	BusinessID             int64  `json:"business_id"`
	BusinessRoleName       string `json:"business_role_name"`
	Active                 bool   `json:"active"`
	IdentityID             *int64 `json:"identity_id"`
	InvitationDateAccepted string `json:"invitation_date_accepted"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
}

// teamMembersListResponse is the literal wire shape observed for the list
// endpoint: a top-level "response" array with "meta" as a sibling, not
// nested inside it the way the rest of the auth family nests its payload.
// FamilyBusiness passes the body through unmodified, so this struct does the
// unwrapping by hand.
type teamMembersListResponse struct {
	Response []TeamMember `json:"response"`
	Meta     PageMeta     `json:"meta"`
}

type teamMemberResponse struct {
	Response TeamMember `json:"response"`
}

// List returns one page of businessID's team members.
//
// inventory: My Team/List Team Members
func (s *TeamMembersService) List(ctx context.Context, businessID BusinessID, opts ...RequestOption) (*Page[TeamMember], error) {
	var resp teamMembersListResponse
	path := "/auth/api/v1/businesses/" + businessID.String() + "/team_members"
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &Page[TeamMember]{
		Items:   resp.Response,
		Page:    resp.Meta.Page,
		Pages:   resp.Meta.Pages,
		PerPage: resp.Meta.PerPage,
		Total:   resp.Meta.Total,
	}, nil
}

// All walks every page of List.
func (s *TeamMembersService) All(ctx context.Context, businessID BusinessID, opts ...RequestOption) iter.Seq2[TeamMember, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[TeamMember], error) {
		return s.List(ctx, businessID, append([]RequestOption{PageNumber(page)}, opts...)...)
	})
}

// Get returns one team member by its UUID.
//
// inventory: My Team/Single Team Member
func (s *TeamMembersService) Get(ctx context.Context, businessID BusinessID, teamMemberUUID string) (*TeamMember, error) {
	var resp teamMemberResponse
	path := "/auth/api/v1/businesses/" + businessID.String() + "/team_members/" + teamMemberUUID
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// InvitationRate is the rate offered to an invitee before they accept and
// become a team member.
type InvitationRate struct {
	Rate       string `json:"rate"`
	ServiceID  int64  `json:"service_id"`
	BusinessID int64  `json:"business_id"`
}

type invitationRatesResponse struct {
	InvitationRates []InvitationRate `json:"invitation_rates"`
}

// InvitationRates lists the per-service rates offered to a business's
// pending invitations.
//
// inventory: Projects/Invitation Rates
func (s *TeamMembersService) InvitationRates(ctx context.Context, businessID BusinessID) ([]InvitationRate, error) {
	var resp invitationRatesResponse
	path := "/comments/business/" + businessID.String() + "/invitation_rates"
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return resp.InvitationRates, nil
}

// TeamMemberRate is one identity's hourly billing rate within a business.
type TeamMemberRate struct {
	Rate       string `json:"rate"`
	IdentityID int64  `json:"identity_id"`
	BusinessID int64  `json:"business_id"`
}

type teamMemberRatesResponse struct {
	TeamMemberRates []TeamMemberRate `json:"team_member_rates"`
}

// Rates lists every team member's billing rate in businessID.
//
// inventory: Projects/Team Member Rates
func (s *TeamMembersService) Rates(ctx context.Context, businessID BusinessID) ([]TeamMemberRate, error) {
	var resp teamMemberRatesResponse
	path := "/comments/business/" + businessID.String() + "/team_member_rates"
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return resp.TeamMemberRates, nil
}

type teamMemberRateResponse struct {
	TeamMemberRate TeamMemberRate `json:"team_member_rate"`
}

// UpdateRate sets identityID's billing rate within businessID. The Postman
// collection lists this endpoint twice, under "My Team" and "Projects";
// both map here.
//
// inventory: My Team/Update Staff Rates
// inventory: Projects/Update Team Member Rate
func (s *TeamMembersService) UpdateRate(ctx context.Context, businessID BusinessID, identityID int64, rate string) (*TeamMemberRate, error) {
	var resp teamMemberRateResponse
	path := fmt.Sprintf("/comments/business/%s/team_member_rate/%d", businessID, identityID)
	body := map[string]map[string]any{
		"team_member_rate": {"identity_id": identityID, "rate": rate},
	}
	if err := s.client.do(ctx, http.MethodPut, path, FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return &resp.TeamMemberRate, nil
}

// InviteRequest is the payload for Invite.
type InviteRequest struct {
	// Capacity is the invitee's role once they accept, e.g. "manager".
	Capacity string `json:"capacity"`
	// ToEmail is the invitee's address.
	ToEmail string `json:"to_email"`
	// InvitableID is the business (or other) resource being invited into.
	InvitableID int64 `json:"invitable_id"`
	// InvitableType names the kind of resource InvitableID addresses, e.g.
	// "business".
	InvitableType string `json:"invitable_type"`
	// Groups optionally scopes the invite to specific project groups.
	Groups []InviteGroup `json:"groups,omitempty"`
}

// InviteGroup scopes an InviteRequest to one project group.
type InviteGroup struct {
	GroupID  int64  `json:"group_id"`
	Capacity string `json:"capacity"`
}

// Invite sends a team invitation. The Postman collection carries no response
// example for this endpoint, so the reply is handed back undecoded: the
// caller can inspect it, but no field is CONFIRMED.
//
// inventory: Projects/Invite Team Member to Project(s)
func (s *TeamMembersService) Invite(ctx context.Context, req *InviteRequest) (map[string]any, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Invite needs a request")
	}
	var resp map[string]any
	if err := s.client.do(ctx, http.MethodPost, "/auth/api/v1/users/invitation", FamilyAuth, req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
