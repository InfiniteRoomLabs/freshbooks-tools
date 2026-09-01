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
	ID                   int64  `json:"id"`
	GroupID              int64  `json:"group_id"`
	Role                 string `json:"role"`
	IdentityID           int64  `json:"identity_id"`
	FirstName            string `json:"first_name"`
	LastName             string `json:"last_name"`
	Email                string `json:"email"`
	Company              string `json:"company"`
	BusinessID           int64  `json:"business_id"`
	Active               bool   `json:"active"`
	UnacknowledgedChange bool   `json:"unacknowledged_change"`
}

// BusinessGroup is the group a business's members belong to, as returned
// alongside both a staff listing (this file) and a newly created business
// (settings.go's Business).
type BusinessGroup struct {
	ID       int64                 `json:"id"`
	Category string                `json:"category"`
	Members  []BusinessGroupMember `json:"members"`
}

// staffListResponse is the auth family's "business + its group" shape,
// after the transport has already peeled off the {"response": ...}
// envelope.
type staffListResponse struct {
	BusinessGroup BusinessGroup `json:"business_group"`
}

func staffBusinessPath(businessID BusinessID) string {
	return "/auth/api/v1/users/business/" + businessID.String()
}

// List returns businessID's staff by way of its business group membership.
// The Postman collection calls this "List Staff", but the payload is the
// business-group-members list the auth family returns for
// /auth/api/v1/users/business/{businessId}, not the accounting Staff
// resource Get/Update/Delete act on -- FreshBooks' own naming conflates the
// two. Use the returned IdentityID as the identity_id time entries need.
// BusinessID is a typed int64 (types.go), so unlike AccountID it needs no
// path-segment validation before interpolation.
//
// inventory: My Team/List Staff
func (s *StaffService) List(ctx context.Context, businessID BusinessID) ([]BusinessGroupMember, error) {
	var resp staffListResponse
	if err := s.client.do(ctx, http.MethodGet, staffBusinessPath(businessID), FamilyAuth, nil, &resp); err != nil {
		return nil, err
	}
	return resp.BusinessGroup.Members, nil
}

// Staff is the deprecated accounting Staff resource: an accounting-family
// user record addressed by account and staff ID. FreshBooks recommends
// TeamMembersService for anything new; this exists for accounts that still
// carry Staff records.
//
// api_token (the account's legacy API token) is deliberately NOT modeled:
// this library's response types never carry a credential, so a caller
// cannot accidentally log, print, or serialize one by handling a Staff
// value. Use (*Client).Do against this same endpoint if that field is
// genuinely needed.
//
// Nullable-string fields the captured Single Staff response returns as
// null (currency_code, note, home_phone, rate) are *string; fields it
// returns as "" (bus_phone, mob_phone, fax, the p_* address fields) stay
// plain string, matching what FreshBooks actually sends for each.
type Staff struct {
	ID                 int64    `json:"id"`
	UserID             int64    `json:"userid"`
	AccountingSystemID string   `json:"accounting_systemid"`
	FirstName          string   `json:"fname"`
	LastName           string   `json:"lname"`
	DisplayName        string   `json:"display_name"`
	Username           string   `json:"username"`
	Email              string   `json:"email"`
	Role               string   `json:"role"`
	Level              int      `json:"level"`
	Organization       string   `json:"organization"`
	Language           string   `json:"language"`
	CurrencyCode       *string  `json:"currency_code"`
	Rate               *string  `json:"rate"`
	Note               *string  `json:"note"`
	BusPhone           string   `json:"bus_phone"`
	HomePhone          *string  `json:"home_phone"`
	MobPhone           string   `json:"mob_phone"`
	Fax                string   `json:"fax"`
	PStreet            string   `json:"p_street"`
	PStreet2           string   `json:"p_street2"`
	PCity              string   `json:"p_city"`
	PProvince          string   `json:"p_province"`
	PCode              string   `json:"p_code"`
	PCountry           string   `json:"p_country"`
	NumLogins          int      `json:"num_logins"`
	LastLogin          DateTime `json:"last_login"`
	SignupDate         DateTime `json:"signup_date"`
	Updated            DateTime `json:"updated"`
	VisState           VisState `json:"vis_state"`
}

type staffResponse struct {
	Staff Staff `json:"staff"`
}

func staffPath(acct AccountID, staffID int64) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/users/staffs/%d", acct, staffID), nil
}

// Get returns one staff record.
//
// inventory: My Team/Single Staff
func (s *StaffService) Get(ctx context.Context, accountID AccountID, staffID int64) (*Staff, error) {
	path, err := staffPath(accountID, staffID)
	if err != nil {
		return nil, err
	}
	var resp staffResponse
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
	path, err := staffPath(accountID, staffID)
	if err != nil {
		return nil, err
	}
	var resp staffResponse
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
	path, err := staffPath(accountID, staffID)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "staff")
}
