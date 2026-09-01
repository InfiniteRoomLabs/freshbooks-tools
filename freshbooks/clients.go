package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// ClientsService is the accounting clients resource.
type ClientsService struct{ client *Client }

// Customer is a FreshBooks client: the customer a business invoices,
// estimates, and bills credits against. Named Customer, not Client, because
// *Client already names this library's API client type; FreshBooks' own API
// uses "customerid" as the field name for this resource's identifier
// throughout the rest of the accounting family (estimates, credit notes),
// so the rename tracks the API's own vocabulary rather than inventing one.
type Customer struct {
	// ID and UserID are the same value under two names; the API returns
	// both.
	ID     int64 `json:"id"`
	UserID int64 `json:"userid"`
	// UUID is the client's stable external identifier.
	UUID string `json:"uuid,omitempty"`
	// AccountingSystemID is the account this client belongs to.
	AccountingSystemID string `json:"accounting_systemid,omitempty"`
	// FirstName, LastName, Organization, and Email are the client's core
	// contact details.
	FirstName    string `json:"fname,omitempty"`
	LastName     string `json:"lname,omitempty"`
	Organization string `json:"organization,omitempty"`
	Email        string `json:"email,omitempty"`
	// Username is the client's portal login, when they have one.
	Username string `json:"username,omitempty"`
	// Role is the client's role, typically "client".
	Role string `json:"role,omitempty"`
	// CurrencyCode and Language are the client's billing currency and
	// preferred language.
	CurrencyCode string `json:"currency_code,omitempty"`
	Language     string `json:"language,omitempty"`
	// VatName and VatNumber record the client's tax registration.
	VatName   string `json:"vat_name,omitempty"`
	VatNumber string `json:"vat_number,omitempty"`
	// Note is a free-text internal note about the client.
	Note string `json:"note,omitempty"`
	// HomePhone, MobilePhone, BusinessPhone, and Fax are the client's phone
	// numbers.
	HomePhone     string `json:"home_phone,omitempty"`
	MobilePhone   string `json:"mob_phone,omitempty"`
	BusinessPhone string `json:"bus_phone,omitempty"`
	Fax           string `json:"fax,omitempty"`
	// CompanyIndustry and CompanySize describe the client's business.
	CompanyIndustry string `json:"company_industry,omitempty"`
	CompanySize     string `json:"company_size,omitempty"`
	// The p_-prefixed fields are the client's billing address; s_-prefixed
	// are their shipping address.
	BillingStreet    string `json:"p_street,omitempty"`
	BillingStreet2   string `json:"p_street2,omitempty"`
	BillingCity      string `json:"p_city,omitempty"`
	BillingProvince  string `json:"p_province,omitempty"`
	BillingCode      string `json:"p_code,omitempty"`
	BillingCountry   string `json:"p_country,omitempty"`
	ShippingStreet   string `json:"s_street,omitempty"`
	ShippingStreet2  string `json:"s_street2,omitempty"`
	ShippingCity     string `json:"s_city,omitempty"`
	ShippingProvince string `json:"s_province,omitempty"`
	ShippingCode     string `json:"s_code,omitempty"`
	ShippingCountry  string `json:"s_country,omitempty"`
	// PrefEmail and PrefGmail are the client's notification preferences.
	PrefEmail bool `json:"pref_email,omitempty"`
	PrefGmail bool `json:"pref_gmail,omitempty"`
	// AllowLateFees and AllowLateNotifications opt the client in to late
	// invoice handling.
	AllowLateFees          bool `json:"allow_late_fees,omitempty"`
	AllowLateNotifications bool `json:"allow_late_notifications,omitempty"`
	// SignupDate, LastLogin, LastActivity, and Updated are account-local
	// timestamps.
	SignupDate   string `json:"signup_date,omitempty"`
	LastLogin    string `json:"last_login,omitempty"`
	LastActivity string `json:"last_activity,omitempty"`
	Updated      string `json:"updated,omitempty"`
	// NumLogins counts the client's portal logins.
	NumLogins int `json:"num_logins,omitempty"`
	// Contacts are the client's secondary contacts, present when requested
	// via Include("recent_contacts") or set on a write.
	Contacts []Contact `json:"contacts,omitempty"`
	// VisState is the client's visibility state.
	VisState VisState `json:"vis_state"`
}

type customerEnvelope struct {
	Client Customer `json:"client"`
}

type customerListEnvelope struct {
	Clients []Customer `json:"clients"`
	PageMeta
}

// ClientWriteRequest is the payload for Create and Update. Every field is
// optional so a caller can send only what it means to set.
type ClientWriteRequest struct {
	FirstName       string    `json:"fname,omitempty"`
	LastName        string    `json:"lname,omitempty"`
	Organization    string    `json:"organization,omitempty"`
	Email           string    `json:"email,omitempty"`
	VatName         *string   `json:"vat_name,omitempty"`
	VatNumber       string    `json:"vat_number,omitempty"`
	Note            *string   `json:"note,omitempty"`
	HomePhone       *string   `json:"home_phone,omitempty"`
	MobilePhone     string    `json:"mob_phone,omitempty"`
	BusinessPhone   string    `json:"bus_phone,omitempty"`
	Fax             string    `json:"fax,omitempty"`
	BillingStreet   string    `json:"p_street,omitempty"`
	BillingStreet2  string    `json:"p_street2,omitempty"`
	BillingCity     string    `json:"p_city,omitempty"`
	BillingProvince string    `json:"p_province,omitempty"`
	BillingCode     string    `json:"p_code,omitempty"`
	BillingCountry  string    `json:"p_country,omitempty"`
	CurrencyCode    string    `json:"currency_code,omitempty"`
	Language        string    `json:"language,omitempty"`
	Contacts        []Contact `json:"contacts,omitempty"`
}

// ClientListOptions filters and paginates List.
type ClientListOptions struct {
	Search  Search
	Page    int
	PerPage int
	Include []string
}

func (o *ClientListOptions) requestOptions() []RequestOption {
	if o == nil {
		return nil
	}
	var opts []RequestOption
	if o.Search != nil {
		opts = append(opts, o.Search)
	}
	if o.Page > 0 {
		opts = append(opts, PageNumber(o.Page))
	}
	if o.PerPage > 0 {
		opts = append(opts, PerPage(o.PerPage))
	}
	if len(o.Include) > 0 {
		opts = append(opts, Include(o.Include...))
	}
	return opts
}

func clientsPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/users/clients", acct)
}

func clientPath(acct AccountID, id int64) string {
	return fmt.Sprintf("/accounting/account/%s/users/clients/%d", acct, id)
}

// List returns one page of clients.
//
// inventory: Clients/List Clients
func (s *ClientsService) List(ctx context.Context, acct AccountID, opts *ClientListOptions) (*Page[Customer], error) {
	var env customerListEnvelope
	if err := s.client.do(ctx, http.MethodGet, clientsPath(acct), FamilyAccounting, nil, &env, opts.requestOptions()...); err != nil {
		return nil, err
	}
	return &Page[Customer]{Items: env.Clients, Page: env.Page, Pages: env.Pages, PerPage: env.PerPage, Total: env.Total}, nil
}

// All walks every page of clients, auto-paginating.
func (s *ClientsService) All(ctx context.Context, acct AccountID, opts *ClientListOptions) iter.Seq2[Customer, error] {
	perPage := 100
	var search Search
	var include []string
	if opts != nil {
		if opts.PerPage > 0 {
			perPage = opts.PerPage
		}
		search, include = opts.Search, opts.Include
	}
	return All(ctx, func(ctx context.Context, page int) (*Page[Customer], error) {
		return s.List(ctx, acct, &ClientListOptions{Search: search, Page: page, PerPage: perPage, Include: include})
	})
}

// Get retrieves a single client.
//
// inventory: Clients/Single Client
func (s *ClientsService) Get(ctx context.Context, acct AccountID, id int64, opts ...RequestOption) (*Customer, error) {
	var env customerEnvelope
	if err := s.client.do(ctx, http.MethodGet, clientPath(acct, id), FamilyAccounting, nil, &env, opts...); err != nil {
		return nil, err
	}
	return &env.Client, nil
}

// Create adds a new client.
//
// inventory: Clients/New Client
func (s *ClientsService) Create(ctx context.Context, acct AccountID, req *ClientWriteRequest) (*Customer, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Clients.Create needs a request")
	}
	body := struct {
		Client *ClientWriteRequest `json:"client"`
	}{req}
	var env customerEnvelope
	if err := s.client.do(ctx, http.MethodPost, clientsPath(acct), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Client, nil
}

// Update changes an existing client's details.
//
// inventory: Clients/Update Client
func (s *ClientsService) Update(ctx context.Context, acct AccountID, id int64, req *ClientWriteRequest) (*Customer, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Clients.Update needs a request")
	}
	body := struct {
		Client *ClientWriteRequest `json:"client"`
	}{req}
	var env customerEnvelope
	if err := s.client.do(ctx, http.MethodPut, clientPath(acct, id), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Client, nil
}

// RemoveAllSecondaryContacts clears every secondary contact from a client in
// one call, by sending an empty contacts array. To remove a single contact,
// use ContactsService.Delete instead.
//
// inventory: Clients/Remove All Secondary Contacts
func (s *ClientsService) RemoveAllSecondaryContacts(ctx context.Context, acct AccountID, id int64) (*Customer, error) {
	body := map[string]any{"client": map[string]any{"contacts": []Contact{}}}
	var env customerEnvelope
	if err := s.client.do(ctx, http.MethodPut, clientPath(acct, id), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Client, nil
}
