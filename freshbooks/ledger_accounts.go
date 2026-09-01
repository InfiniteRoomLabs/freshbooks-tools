package freshbooks

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// LedgerAccount is one entry in a business's chart of accounts. Unlike the
// rest of the accounting family it is addressed by BusinessUUID, not
// AccountID, and its responses are flat -- see the transport's ledger-family
// classification in client.go.
type LedgerAccount struct {
	// UUID identifies the account.
	UUID string `json:"uuid"`
	// Name is the account's display name.
	Name string `json:"name"`
	// Number is the account's chart-of-accounts number, e.g. "1000".
	Number string `json:"number"`
	// Description is a free-text note on the account.
	Description string `json:"description"`
	// Type is the top-level category, e.g. "asset", "liability", "equity".
	Type string `json:"type"`
	// SubType is the account's sub-type, e.g. "Cash & Bank".
	SubType string `json:"sub_type"`
	// SystemAccountName names the built-in account this one corresponds to,
	// when FreshBooks created it automatically.
	SystemAccountName string `json:"system_account_name,omitempty"`
	// ParentAccount is the parent account's UUID, nil for a top-level
	// account.
	ParentAccount *string `json:"parent_account"`
	// SubAccounts lists the UUIDs of this account's children.
	SubAccounts []string `json:"sub_accounts"`
	// AutoCreated reports whether FreshBooks created this account rather
	// than the user.
	AutoCreated bool `json:"auto_created"`
	// State is the account's lifecycle state, e.g. "active".
	State string `json:"state"`
	// UpdatedAt is when the account was last changed.
	UpdatedAt time.Time `json:"updated_at"`
}

// LedgerAccountCreateRequest is the payload for LedgerAccountsService.Create.
type LedgerAccountCreateRequest struct {
	// Name is the account's display name.
	Name string `json:"name"`
	// Number is the account's chart-of-accounts number.
	Number string `json:"number"`
	// Description is a free-text note on the account.
	Description string `json:"description,omitempty"`
	// SubType chooses the account's sub-type, e.g. "Cash & Bank".
	SubType string `json:"sub_type"`
	// ParentAccount is the parent account's UUID, when this is a
	// sub-account.
	ParentAccount *string `json:"parent_account,omitempty"`
}

// LedgerAccountUpdateRequest is the payload for LedgerAccountsService.Update.
// FreshBooks expects the full account back, UUID included.
type LedgerAccountUpdateRequest struct {
	// UUID identifies the account being updated.
	UUID string `json:"uuid"`
	// Name is the account's display name.
	Name string `json:"name"`
	// Number is the account's chart-of-accounts number.
	Number string `json:"number"`
	// Description is a free-text note on the account.
	Description string `json:"description,omitempty"`
	// Type is the top-level category. The API accepts it back even though
	// it never changes in practice.
	Type string `json:"type"`
	// SubType chooses the account's sub-type.
	SubType string `json:"sub_type"`
	// ParentAccount is the parent account's UUID, or nil to leave it
	// top-level.
	ParentAccount *string `json:"parent_account"`
}

// ledgerAccountEnvelope is the flat {"data": ...} shape every ledger-account
// endpoint answers with.
type ledgerAccountEnvelope struct {
	Data LedgerAccount `json:"data"`
}

// LedgerAccountSubType is one entry in the sub-type taxonomy a ledger
// account's SubType is chosen from.
//
// The taxonomy endpoints (Types, SubTypes, SubType) carry no example
// response in the Postman collection and no public FreshBooks docs page.
// This shape is INFERRED from the "type" and "sub_type" strings ledger
// accounts themselves return, not from an observed response; treat it as
// provisional until a live call confirms it.
type LedgerAccountSubType struct {
	// ID identifies the sub-type, addressable via SubType.
	ID string `json:"id"`
	// Name is the sub-type's display name, e.g. "Cash & Bank".
	Name string `json:"name"`
	// Type is the parent account type this sub-type belongs to, e.g.
	// "asset".
	Type string `json:"type"`
}

// Create adds a ledger account to biz's chart of accounts.
//
// inventory: Accounting/Accounts/Create Account
func (s *LedgerAccountsService) Create(ctx context.Context, biz BusinessUUID, req *LedgerAccountCreateRequest) (*LedgerAccount, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: LedgerAccounts.Create needs a request")
	}
	path := "/accounting/businesses/" + string(biz) + "/ledger_accounts/accounts"
	var resp ledgerAccountEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, familyForPath(path), req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// List returns every ledger account in biz's chart of accounts. The API
// answers a flat array with no pagination envelope.
//
// inventory: Accounting/Accounts/List Accounts
func (s *LedgerAccountsService) List(ctx context.Context, biz BusinessUUID) ([]LedgerAccount, error) {
	path := "/accounting/businesses/" + string(biz) + "/ledger_accounts/accounts"
	var resp struct {
		Data []LedgerAccount `json:"data"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, familyForPath(path), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// Get returns one ledger account by UUID.
//
// inventory: Accounting/Accounts/Single Account
func (s *LedgerAccountsService) Get(ctx context.Context, biz BusinessUUID, accountUUID string) (*LedgerAccount, error) {
	path := "/accounting/businesses/" + string(biz) + "/ledger_accounts/accounts/" + accountUUID
	var resp ledgerAccountEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, familyForPath(path), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Update replaces a ledger account's editable fields.
//
// inventory: Accounting/Accounts/Update Account
func (s *LedgerAccountsService) Update(ctx context.Context, biz BusinessUUID, accountUUID string, req *LedgerAccountUpdateRequest) (*LedgerAccount, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: LedgerAccounts.Update needs a request")
	}
	path := "/accounting/businesses/" + string(biz) + "/ledger_accounts/accounts/" + accountUUID
	var resp ledgerAccountEnvelope
	if err := s.client.do(ctx, http.MethodPut, path, familyForPath(path), req, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// Types returns the account-type taxonomy (e.g. "asset", "liability")
// ledger accounts choose their Type from. The endpoint takes no scope ID.
//
// INFERRED shape: see LedgerAccountSubType's doc comment.
//
// inventory: Accounting/Accounts/List Account types
func (s *LedgerAccountsService) Types(ctx context.Context) ([]string, error) {
	const path = "/accounting/ledger_accounts/types"
	var resp struct {
		Data []string `json:"data"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, familyForPath(path), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// SubTypes returns the sub-type taxonomy ledger accounts choose their
// SubType from. The endpoint takes no scope ID.
//
// INFERRED shape: see LedgerAccountSubType's doc comment.
//
// inventory: Accounting/Accounts/List Sub types
func (s *LedgerAccountsService) SubTypes(ctx context.Context) ([]LedgerAccountSubType, error) {
	const path = "/accounting/ledger_accounts/sub_types"
	var resp struct {
		Data []LedgerAccountSubType `json:"data"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, familyForPath(path), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// SubType returns one sub-type by ID.
//
// INFERRED shape: see LedgerAccountSubType's doc comment.
//
// inventory: Accounting/Accounts/Single Sub type
func (s *LedgerAccountsService) SubType(ctx context.Context, id string) (*LedgerAccountSubType, error) {
	path := "/accounting/ledger_accounts/sub_types/" + id
	var resp struct {
		Data LedgerAccountSubType `json:"data"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, familyForPath(path), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
