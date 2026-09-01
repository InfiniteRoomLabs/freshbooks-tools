package tools

import "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"

// Scope carries the account/business identifiers a tool call resolves
// against: the server's configured defaults, overridden field by field by
// whatever a tool input supplies.
type Scope struct {
	AccountID    freshbooks.AccountID
	BusinessID   freshbooks.BusinessID
	BusinessUUID freshbooks.BusinessUUID
}

// AcctScope is embedded by every tool input scoped to an accounting-family
// account. The field is optional on the wire: when empty, Register's
// default scope supplies it, and the tool fails with a named-field IsError
// result when neither is present.
type AcctScope struct {
	AccountID string `json:"account_id,omitempty" jsonschema:"FreshBooks account id; falls back to the server's configured default when omitted"`
}

func (s AcctScope) accountIDOverride() string { return s.AccountID }

// BizScope is embedded by every tool input scoped to a business-family
// business. See AcctScope's doc comment for the fallback rule.
type BizScope struct {
	BusinessID int64 `json:"business_id,omitempty" jsonschema:"FreshBooks business id; falls back to the server's configured default when omitted"`
}

func (s BizScope) businessIDOverride() int64 { return s.BusinessID }

// UUIDScope is embedded by every tool input scoped to a ledger-accounts
// business UUID. See AcctScope's doc comment for the fallback rule.
type UUIDScope struct {
	BusinessUUID string `json:"business_uuid,omitempty" jsonschema:"FreshBooks business UUID; falls back to the server's configured default when omitted"`
}

func (s UUIDScope) businessUUIDOverride() string { return s.BusinessUUID }

type accountOverrider interface{ accountIDOverride() string }
type businessOverrider interface{ businessIDOverride() int64 }
type businessUUIDOverrider interface{ businessUUIDOverride() string }

// resolveScope merges defaults with any per-request overrides embedded in
// in (via AcctScope, BizScope, and/or UUIDScope), and reports the name of
// the first required scope field that still resolved empty, or "" when
// every scope field the input type declares is present. Which scope fields
// are "required" is entirely a function of which of the three marker
// structs In embeds -- a tool that embeds none (e.g. identity_whoami) never
// reports a missing field.
func resolveScope[In any](in In, defaults Scope) (Scope, string) {
	resolved := defaults
	var missing string

	if ov, ok := any(in).(accountOverrider); ok {
		if v := ov.accountIDOverride(); v != "" {
			resolved.AccountID = freshbooks.AccountID(v)
		}
		if resolved.AccountID == "" && missing == "" {
			missing = "account_id"
		}
	}
	if ov, ok := any(in).(businessOverrider); ok {
		if v := ov.businessIDOverride(); v != 0 {
			resolved.BusinessID = freshbooks.BusinessID(v)
		}
		if resolved.BusinessID == 0 && missing == "" {
			missing = "business_id"
		}
	}
	if ov, ok := any(in).(businessUUIDOverrider); ok {
		if v := ov.businessUUIDOverride(); v != "" {
			resolved.BusinessUUID = freshbooks.BusinessUUID(v)
		}
		if resolved.BusinessUUID == "" && missing == "" {
			missing = "business_uuid"
		}
	}
	return resolved, missing
}
