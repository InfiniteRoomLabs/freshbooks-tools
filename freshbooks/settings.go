package freshbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Settings/Businesses and Settings/Developer are auth-family endpoints with
// no natural home among the 36 pre-declared services in services.go: they
// manage the account's businesses and OAuth applications, not a resource
// any other service already models. IdentityService is the closest fit --
// it already owns Membership, the closest existing type to "business" -- so
// these methods land here rather than under a newly declared service.
// Flagged for the review gate per the batch c work order.

// BusinessAddress is a business's mailing address as returned by the
// business-management endpoints.
type BusinessAddress struct {
	ID         int64   `json:"id"`
	Street     *string `json:"street"`
	City       *string `json:"city"`
	Province   *string `json:"province"`
	Country    string  `json:"country"`
	PostalCode *string `json:"postal_code"`
}

// Business is a business record as returned by AddBusiness -- richer than
// Membership, which only carries the identifiers other services need.
//
// PhoneNumber is a pointer: the captured response returns null.
// BusinessClients is an empty array in the captured response with no
// non-empty example anywhere in the collection, so its element shape is
// undetermined; it decodes as raw JSON rather than a guessed struct.
type Business struct {
	ID              int64             `json:"id"`
	Name            string            `json:"name"`
	AccountID       string            `json:"account_id"`
	BusinessGroup   BusinessGroup     `json:"business_group"`
	DateFormat      string            `json:"date_format"`
	Address         *BusinessAddress  `json:"address"`
	PhoneNumber     *string           `json:"phone_number"`
	BusinessClients []json.RawMessage `json:"business_clients,omitempty"`
}

// BusinessCreateRequest is the payload for AddBusiness.
type BusinessCreateRequest struct {
	Name              string            `json:"name"`
	DateFormat        string            `json:"date_format,omitempty"`
	AddressAttributes map[string]string `json:"address_attributes,omitempty"`
}

// AddBusiness provisions a new business under the current identity.
//
// inventory: Settings/Businesses/Add Business
func (s *IdentityService) AddBusiness(ctx context.Context, req *BusinessCreateRequest) (*Business, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: AddBusiness needs a request")
	}
	var resp Business
	if err := s.client.do(ctx, http.MethodPost, "/auth/api/v1/users/business", FamilyAuth, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteBusiness removes a business. Destructive and irreversible: a CLI or
// MCP surface built on this method must require explicit confirmation and
// must not expose it as an unattended tool. FreshBooks refuses this with a
// 422 while the business carries an active subscription; cancel it first
// with DeleteBusinessSubscription.
//
// inventory: Settings/Businesses/Delete Business
func (s *IdentityService) DeleteBusiness(ctx context.Context, businessID BusinessID) error {
	path := "/auth/api/v1/users/business/" + businessID.String()
	return s.client.do(ctx, http.MethodDelete, path, FamilyAuth, nil, nil)
}

// DeleteBusinessSubscription cancels accountID's billing subscription, the
// prerequisite DeleteBusiness enforces. Destructive and irreversible: a CLI
// or MCP surface built on this method must require explicit confirmation
// and must not expose it as an unattended tool.
//
// The Postman collection sources this request from my.freshbooks.com --
// FreshBooks' internal host -- rather than the public api.freshbooks.com
// the rest of the collection uses. The inventory tool rewrites the host to
// public; this method implements against that rewritten path
// (/auth/api/v1/billing/account/{account_id}/subscription). INFERRED from
// that Postman example alone, never confirmed live.
//
// inventory: Settings/Businesses/Delete Business - Subscription
func (s *IdentityService) DeleteBusinessSubscription(ctx context.Context, accountID AccountID) error {
	if err := pathSegment(string(accountID)); err != nil {
		return err
	}
	path := "/auth/api/v1/billing/account/" + string(accountID) + "/subscription"
	return s.client.do(ctx, http.MethodDelete, path, FamilyAuth, nil, nil)
}

// PaymentsProvisionRequest is the payload for ProvisionPayments.
type PaymentsProvisionRequest struct {
	FirstName              string `json:"fname"`
	LastName               string `json:"lname"`
	Email                  string `json:"email"`
	OrgName                string `json:"orgname"`
	RedirectBaseURI        string `json:"redirect_base_uri"`
	Country                string `json:"country"`
	AllowMultipleProvision bool   `json:"allow_multiple_provision,omitempty"`
}

// ProvisionPayments enrolls accountID in FreshBooks Payments. The Postman
// example returns 201 with a null body, so there is nothing to decode; a
// nil error means the enrollment succeeded.
//
// This request's family is "payments" per the inventory classifier, which
// the transport currently folds into FamilyBusiness as an unverified
// fallback (spec 5.1's existing callout, owned by batch d). A null
// success body sidesteps the question here since there is no envelope to
// unwrap either way; if a future confirmation reclassifies "payments",
// this call needs no change.
//
// inventory: Settings/Businesses/Provision FreshBooks Payments
func (s *IdentityService) ProvisionPayments(ctx context.Context, accountID AccountID, req *PaymentsProvisionRequest) error {
	if req == nil {
		return fmt.Errorf("freshbooks: ProvisionPayments needs a request")
	}
	if err := pathSegment(string(accountID)); err != nil {
		return err
	}
	path := "/payments/account/" + string(accountID) + "/gateway/fbpay"
	return s.client.do(ctx, http.MethodPost, path, FamilyBusiness, req, nil)
}

// Application is a registered OAuth application (a "partner application" in
// FreshBooks' terms). Its response shape is INFERRED: the Postman
// collection carries no genuine response example for any of the three
// Developer endpoints (Get all applications' example is a copy-pasted
// Identity Info response, not this endpoint's actual shape) -- these fields
// come from the Create/Modify request bodies, which is the only evidence
// available.
//
// Application deliberately implements fmt.Stringer with a redacted
// rendering, mirroring auth.Token, so printing one with %v or %s cannot
// leak the OAuth client secret. An MCP tool surfacing Application must
// strip ClientSecret from its output explicitly (Phase 3 inherits this
// constraint in writing): a stateless, model-facing tool result is not the
// place for a live credential.
type Application struct {
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	Name         string `json:"name"`
	RedirectURI  string `json:"redirect_uri"`
	Description  string `json:"description,omitempty"`
	WebsiteURL   string `json:"website_url,omitempty"`
	SettingsURL  string `json:"settings_url,omitempty"`
	LogoPublicID string `json:"logo_public_id,omitempty"`
}

// String renders the application with its client secret redacted.
func (a Application) String() string {
	return fmt.Sprintf("freshbooks.Application{ClientID: %q, ClientSecret: redacted, Name: %q, RedirectURI: %q}",
		a.ClientID, a.Name, a.RedirectURI)
}

// ApplicationCreateRequest is the payload for CreateApplication.
type ApplicationCreateRequest struct {
	Name         string `json:"name"`
	RedirectURI  string `json:"redirect_uri"`
	Description  string `json:"description,omitempty"`
	WebsiteURL   string `json:"website_url,omitempty"`
	SettingsURL  string `json:"settings_url,omitempty"`
	LogoPublicID string `json:"logo_public_id,omitempty"`
}

// CreateApplication registers a new OAuth application. See Application's
// doc comment: the response shape is INFERRED, not confirmed live.
//
// inventory: Settings/Developer/Create new application
func (s *IdentityService) CreateApplication(ctx context.Context, req *ApplicationCreateRequest) (*Application, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: CreateApplication needs a request")
	}
	var resp Application
	if err := s.client.do(ctx, http.MethodPost, "/auth/api/v1/partners/applications", FamilyAuth, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Applications lists the current identity's registered OAuth applications.
// See Application's doc comment: the response shape is INFERRED, not
// confirmed live -- the Postman example for this request is a mislabeled
// copy of an unrelated endpoint's response and was not used as evidence.
//
// inventory: Settings/Developer/Get all applications
func (s *IdentityService) Applications(ctx context.Context) ([]Application, error) {
	var resp []Application
	if err := s.client.do(ctx, http.MethodGet, "/auth/api/v1/partners/applications", FamilyAuth, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ApplicationUpdateRequest is the payload for UpdateApplication. The
// captured Postman body is a full-replace PUT -- all seven fields are sent
// every time, including the four optional ones -- so none of them carry
// omitempty: a caller clearing Description to "" must have that reach the
// server as "", not vanish from the request. FreshBooks also requires
// ClientSecret on this call (the Postman example always includes it),
// unlike Create, which never has one yet.
type ApplicationUpdateRequest struct {
	Name         string `json:"name"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
	Description  string `json:"description"`
	WebsiteURL   string `json:"website_url"`
	SettingsURL  string `json:"settings_url"`
	LogoPublicID string `json:"logo_public_id"`
}

// String renders the update request with its client secret redacted.
func (r ApplicationUpdateRequest) String() string {
	return fmt.Sprintf("freshbooks.ApplicationUpdateRequest{Name: %q, ClientSecret: redacted, RedirectURI: %q}",
		r.Name, r.RedirectURI)
}

// UpdateApplication edits a registered OAuth application. See Application's
// doc comment: the response shape is INFERRED, not confirmed live.
//
// inventory: Settings/Developer/Modify existing application
func (s *IdentityService) UpdateApplication(ctx context.Context, clientID string, req *ApplicationUpdateRequest) (*Application, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: UpdateApplication needs a request")
	}
	if err := pathSegment(clientID); err != nil {
		return nil, err
	}
	var resp Application
	path := "/auth/api/v1/partners/applications/" + clientID
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAuth, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
