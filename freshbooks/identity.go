package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// IdentityService resolves who the current token belongs to, and which
// account and business identifiers its memberships expose. Every other
// service needs one of those identifiers, so this is normally the first call
// a program makes.
type IdentityService struct{ client *Client }

// Membership is one business the authenticated user belongs to, carrying
// every identifier the rest of the API needs. AccountID addresses the
// accounting family, BusinessID the business-scoped family, and BusinessUUID
// the ledger-accounts endpoints; they are three spellings of one business and
// are never interchangeable.
type Membership struct {
	// AccountID addresses /accounting/account/{account_id}/....
	AccountID AccountID `json:"account_id"`
	// BusinessID addresses /projects/business/{business_id}/... and its
	// siblings.
	BusinessID BusinessID `json:"business_id"`
	// BusinessUUID addresses
	// /accounting/businesses/{business_uuid}/ledger_accounts/....
	BusinessUUID BusinessUUID `json:"business_uuid"`
	// Name is the business's display name.
	Name string `json:"name"`
	// Role is the user's role in the business, e.g. "owner".
	Role string `json:"role"`
}

// User is the authenticated identity behind a token.
type User struct {
	// ID is the identity's numeric id.
	ID int64 `json:"id"`
	// Email is the identity's email address.
	Email string `json:"email"`
	// FirstName and LastName are the identity's given and family names.
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	// Memberships are the businesses this identity belongs to.
	Memberships []Membership `json:"memberships"`
}

// identityResponse is the auth family's users/me payload, after the
// transport has peeled off the {"response": ...} envelope.
type identityResponse struct {
	ID                  int64  `json:"id"`
	Email               string `json:"email"`
	FirstName           string `json:"first_name"`
	LastName            string `json:"last_name"`
	BusinessMemberships []struct {
		Role     string `json:"role"`
		Business struct {
			ID           int64  `json:"id"`
			AccountID    string `json:"account_id"`
			BusinessUUID string `json:"business_uuid"`
			Name         string `json:"name"`
		} `json:"business"`
	} `json:"business_memberships"`
}

func (r identityResponse) user() *User {
	u := &User{ID: r.ID, Email: r.Email, FirstName: r.FirstName, LastName: r.LastName}
	for _, m := range r.BusinessMemberships {
		u.Memberships = append(u.Memberships, Membership{
			AccountID:    AccountID(m.Business.AccountID),
			BusinessID:   BusinessID(m.Business.ID),
			BusinessUUID: BusinessUUID(m.Business.BusinessUUID),
			Name:         m.Business.Name,
			Role:         m.Role,
		})
	}
	return u
}

// Me returns the memberships of the identity the current token belongs to.
// Use Whoami when the identity's own fields are wanted too.
//
// The Postman collection lists this one endpoint twice, under two names;
// both map here.
//
// inventory: Authorization/Identity Info Call
// inventory: Authorization/List User
func (s *IdentityService) Me(ctx context.Context) ([]Membership, error) {
	u, err := s.Whoami(ctx)
	if err != nil {
		return nil, err
	}
	return u.Memberships, nil
}

// Whoami returns the full identity behind the current token, memberships
// included.
func (s *IdentityService) Whoami(ctx context.Context) (*User, error) {
	var resp identityResponse
	if err := s.client.do(ctx, http.MethodGet, "/auth/api/v1/users/me", FamilyAuth, nil, &resp); err != nil {
		return nil, err
	}
	return resp.user(), nil
}

// RegisterRequest is the payload for Register. Password is a credential:
// keep it out of logs, and prefer sending users through the FreshBooks
// signup flow instead of calling this at all.
type RegisterRequest struct {
	// ID is the new user's login, which FreshBooks expects to equal Email.
	ID string `json:"id"`
	// Email is the new user's email address.
	Email string `json:"email"`
	// Password is the new user's password.
	Password string `json:"password"`
	// CompanyName names the business to create, when set.
	CompanyName *string `json:"company_name,omitempty"`
	// Country is the business's country, e.g. "Canada".
	Country string `json:"country,omitempty"`
	// CurrencyCode is the business's ISO 4217 currency.
	CurrencyCode string `json:"currencyCode,omitempty"`
	// SkipSystem and SkipBusiness suppress the default system and business
	// FreshBooks would otherwise provision.
	SkipSystem   bool `json:"skip_system,omitempty"`
	SkipBusiness bool `json:"skip_business,omitempty"`
	// SendConfirmationNotification asks FreshBooks to email the new user.
	SendConfirmationNotification bool `json:"send_confirmation_notification,omitempty"`
}

// Register provisions a brand-new FreshBooks user. It is the only endpoint in
// the collection that creates an identity rather than acting on one, and it
// exists here for parity; almost every caller wants the hosted signup flow.
//
// inventory: Authorization/Register as a new user
func (s *IdentityService) Register(ctx context.Context, req *RegisterRequest) (*User, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Register needs a request")
	}
	var resp identityResponse
	if err := s.client.do(ctx, http.MethodPost, "/auth/api/v1/smux/registrations", FamilyAuth, req, &resp); err != nil {
		return nil, err
	}
	return resp.user(), nil
}
