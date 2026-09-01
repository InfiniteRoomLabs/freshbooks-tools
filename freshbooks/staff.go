package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// BusinessGroupMember is one member of a business's group, as returned by
// listing a business's staff through the auth family. It carries the
// identity_id every time-tracking write needs, alongside the role that
// governs what that identity can do in the business.
type BusinessGroupMember struct {
	ID         int64  `json:"id"`
	GroupID    int64  `json:"group_id"`
	Role       string `json:"role"`
	IdentityID int64  `json:"identity_id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	Company    string `json:"company"`
	BusinessID int64  `json:"business_id"`
	Active     bool   `json:"active"`
}

// staffListResponse is the auth family's "business + its group" shape,
// after the transport has already peeled off the {"response": ...}
// envelope. The member list sits one level deeper still, under
// "business_group.members".
type staffListResponse struct {
	BusinessGroup struct {
		Members []BusinessGroupMember `json:"members"`
	} `json:"business_group"`
}

// List returns businessID's staff by way of its business group membership.
// The Postman collection calls this "List Staff", but the payload is the
// business-group-members list the auth family returns for
// /auth/api/v1/users/business/{businessId}, not the accounting Staff
// resource Get/Update/Delete act on -- FreshBooks' own naming conflates the
// two. Use the returned IdentityID as the identity_id time entries need.
//
// inventory: My Team/List Staff
func (s *StaffService) List(ctx context.Context, businessID BusinessID) ([]BusinessGroupMember, error) {
	var resp staffListResponse
	path := "/auth/api/v1/users/business/" + businessID.String()
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAuth, nil, &resp); err != nil {
		return nil, err
	}
	return resp.BusinessGroup.Members, nil
}

// Staff is the deprecated accounting Staff resource: an accounting-family
// user record addressed by account and staff ID. FreshBooks recommends
// TeamMembersService for anything new; this exists for accounts that still
// carry Staff records.
type Staff struct {
	ID           int64    `json:"id"`
	UserID       int64    `json:"userid"`
	FirstName    string   `json:"fname"`
	LastName     string   `json:"lname"`
	DisplayName  string   `json:"display_name"`
	Username     string   `json:"username"`
	Email        string   `json:"email"`
	Role         string   `json:"role"`
	Level        int      `json:"level"`
	Organization string   `json:"organization"`
	Language     string   `json:"language"`
	CurrencyCode string   `json:"currency_code"`
	Rate         *string  `json:"rate"`
	Note         string   `json:"note"`
	BusPhone     string   `json:"bus_phone"`
	HomePhone    string   `json:"home_phone"`
	MobPhone     string   `json:"mob_phone"`
	Fax          string   `json:"fax"`
	PStreet      string   `json:"p_street"`
	PStreet2     string   `json:"p_street2"`
	PCity        string   `json:"p_city"`
	PProvince    string   `json:"p_province"`
	PCode        string   `json:"p_code"`
	PCountry     string   `json:"p_country"`
	NumLogins    int      `json:"num_logins"`
	LastLogin    string   `json:"last_login"`
	SignupDate   string   `json:"signup_date"`
	Updated      string   `json:"updated"`
	VisState     VisState `json:"vis_state"`
}

type staffResponse struct {
	Staff Staff `json:"staff"`
}

// Get returns one staff record.
//
// inventory: My Team/Single Staff
func (s *StaffService) Get(ctx context.Context, accountID AccountID, staffID int64) (*Staff, error) {
	var resp staffResponse
	path := fmt.Sprintf("/accounting/account/%s/users/staffs/%d", accountID, staffID)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Staff, nil
}

// StaffUpdateRequest is the payload for Update. Every field is optional;
// only set ones are sent.
type StaffUpdateRequest struct {
	FirstName *string `json:"fname,omitempty"`
	LastName  *string `json:"lname,omitempty"`
	Note      *string `json:"note,omitempty"`
	BusPhone  *string `json:"bus_phone,omitempty"`
}

// Update edits a staff record.
//
// inventory: My Team/Update Staff
func (s *StaffService) Update(ctx context.Context, accountID AccountID, staffID int64, req *StaffUpdateRequest) (*Staff, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Update needs a request")
	}
	var resp staffResponse
	path := fmt.Sprintf("/accounting/account/%s/users/staffs/%d", accountID, staffID)
	body := map[string]*StaffUpdateRequest{"staff": req}
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Staff, nil
}

// Delete soft-deletes a staff record by setting its vis_state, the same PUT
// endpoint Update uses with a fixed body -- the accounting family has no
// staff-specific DELETE verb.
//
// inventory: My Team/Delete Staff
func (s *StaffService) Delete(ctx context.Context, accountID AccountID, staffID int64) error {
	path := fmt.Sprintf("/accounting/account/%s/users/staffs/%d", accountID, staffID)
	body := map[string]map[string]VisState{"staff": {"vis_state": VisStateDeleted}}
	return s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, nil)
}
