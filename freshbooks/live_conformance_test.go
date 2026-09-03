//go:build live

// Phase 7's live-conformance suite: one test per fact the design spec
// carried as INFERRED or docs-only, each making the real call and asserting
// the shape the matching capture under testdata/seed/ shows. Read-only:
// nothing here creates, updates, or deletes. Same gate as live_test.go (the
// "live" build tag plus FRESHBOOKS_LIVE=1), and the same discipline about
// account data -- assertions check presence and shape, never values, so a
// failure message cannot leak a real id, name, or address.
package freshbooks_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// liveScope returns the first membership of the authorized account: the
// account id, business id, and business uuid every other fact needs.
func liveScope(t *testing.T, c *freshbooks.Client, ctx context.Context) freshbooks.Membership {
	t.Helper()
	ms, err := c.Identity.Me(ctx)
	if err != nil {
		t.Fatalf("Identity.Me: %v", err)
	}
	if len(ms) == 0 {
		t.Fatal("the authorized account has no business memberships")
	}
	return ms[0]
}

// liveCtx builds the per-test context.
func liveCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 60*time.Second)
}

// TestLiveExpenseVendors is fact J1: Phase 2 inferred that
// Expenses/Expense Vendors answers a bare string array under "vendors"
// (the Postman collection carries no example). It does not: the payload is
// the paginated accounting shape, and each entry is a one-key object.
func TestLiveExpenseVendors(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := liveCtx(t)
	defer cancel()
	m := liveScope(t, c, ctx)

	vendors, err := c.Expenses.Vendors(ctx, m.AccountID)
	if err != nil {
		t.Fatalf("Expenses.Vendors: %v", err)
	}
	if len(vendors) == 0 {
		t.Skip("the authorized account has no expense vendors; nothing to assert about the entry shape")
	}
	for i, v := range vendors {
		if v == "" {
			t.Errorf("vendor %d decoded empty -- the wire entry is {\"vendor\": \"...\"}, not a bare string", i)
		}
	}
	t.Logf("decoded %d vendor name(s) from the paginated envelope", len(vendors))
}

// TestLiveGateways is fact E: /payments/ was classified as the business
// (flat, un-enveloped) family from a Postman example only. It also covers
// the shape correction this phase found -- an account onboarded through
// FreshBooks Payments answers under "stripe_unified", with "stripe": null
// and no "fbpay" key at all.
func TestLiveGateways(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := liveCtx(t)
	defer cancel()
	m := liveScope(t, c, ctx)

	conns, err := c.Gateways.Get(ctx, m.AccountID)
	if err != nil {
		t.Fatalf("Gateways.Get: %v", err)
	}
	// Decoding at all is the family assertion: an accounting-family
	// classification would have looked for a {"response":{"result":...}}
	// wrapper that is not there and yielded nothing.
	if len(conns) == 0 {
		t.Skip("the authorized account has no gateway connections; family confirmed, shape not asserted")
	}
	var populated int
	for _, conn := range conns {
		if conn.FBPay != nil {
			populated++
		}
		if conn.Stripe != nil {
			populated++
		}
		if su := conn.StripeUnified; su != nil {
			populated++
			if su.StripeAccountID == "" || su.PublishableKey == "" {
				t.Error("stripe_unified decoded without its account id or publishable key")
			}
			if su.AccountStatus == "" {
				t.Error("stripe_unified decoded without an account_status")
			}
			if len(su.Capabilities) == 0 {
				t.Error("stripe_unified decoded without capabilities")
			}
			if su.StripeAccountUpdatedAt.IsZero() {
				t.Error("stripe_account_updated_at did not parse")
			}
		}
	}
	if populated == 0 {
		t.Error("every gateway field decoded nil -- the connection shape is not modelled")
	}
}

// TestLiveLedgerAccounts is fact F: the ledger family answers flat
// {"data": ...} (Phase 2 reclassified it off the accounting envelope on
// Postman examples alone), and the two taxonomy endpoints -- which have no
// Postman example and no public docs page -- had no evidence for their
// payload shape at all.
func TestLiveLedgerAccounts(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := liveCtx(t)
	defer cancel()
	m := liveScope(t, c, ctx)

	t.Run("list answers flat data", func(t *testing.T) {
		accounts, err := c.LedgerAccounts.List(ctx, m.BusinessUUID)
		if err != nil {
			t.Fatalf("LedgerAccounts.List: %v", err)
		}
		if len(accounts) == 0 {
			t.Fatal("the chart of accounts is empty; every FreshBooks business has auto-created accounts")
		}
		for i, a := range accounts {
			if a.UUID == "" || a.Type == "" {
				t.Fatalf("account %d decoded without a uuid or type", i)
			}
			if a.UpdatedAt.IsZero() {
				t.Fatalf("account %d: updated_at did not parse", i)
			}
		}
		t.Logf("decoded %d ledger account(s)", len(accounts))
	})

	t.Run("types are one-key objects", func(t *testing.T) {
		types, err := c.LedgerAccounts.Types(ctx)
		if err != nil {
			t.Fatalf("LedgerAccounts.Types: %v", err)
		}
		var names []string
		for _, ty := range types {
			if ty.Name == "" {
				t.Fatal("a type decoded with an empty name -- the entries are {\"name\": \"...\"} objects")
			}
			names = append(names, ty.Name)
		}
		for _, want := range []string{"asset", "equity", "expense", "income", "liability"} {
			if !slices.Contains(names, want) {
				t.Errorf("type %q missing from %v", want, names)
			}
		}
	})

	t.Run("sub-types carry a numeric id and a base number", func(t *testing.T) {
		subTypes, err := c.LedgerAccounts.SubTypes(ctx)
		if err != nil {
			t.Fatalf("LedgerAccounts.SubTypes: %v", err)
		}
		if len(subTypes) == 0 {
			t.Fatal("the sub-type taxonomy is empty")
		}
		for i, st := range subTypes {
			if st.ID == 0 || st.Type == "" || st.Name == "" || st.BaseNumber == "" {
				t.Fatalf("sub-type %d decoded incomplete: %+v", i, st)
			}
		}

		first := subTypes[0]
		one, err := c.LedgerAccounts.SubType(ctx, strconv.FormatInt(first.ID, 10))
		if err != nil {
			t.Fatalf("LedgerAccounts.SubType: %v", err)
		}
		if *one != first {
			t.Errorf("the single sub-type differs from its list entry: %+v vs %+v", one, first)
		}
	})
}

// TestLiveStaffFields is fact P: StaffService.List decodes the auth
// family's "business + its group" payload and returns only the members.
// Two member keys the Postman example does not have -- identity_uuid and
// language -- were being dropped; the sibling business fields are dropped
// deliberately (see staffListResponse).
func TestLiveStaffFields(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := liveCtx(t)
	defer cancel()
	m := liveScope(t, c, ctx)

	members, err := c.Staff.List(ctx, m.BusinessID)
	if err != nil {
		t.Fatalf("Staff.List: %v", err)
	}
	if len(members) == 0 {
		t.Fatal("the business has no group members; the authorized identity is itself one")
	}
	for i, member := range members {
		if member.ID == 0 || member.GroupID == 0 || member.IdentityID == 0 {
			t.Errorf("member %d decoded without its ids", i)
		}
		if member.IdentityUUID == "" {
			t.Errorf("member %d: identity_uuid dropped", i)
		}
		if member.Language == "" {
			t.Errorf("member %d: language dropped", i)
		}
		if member.Role == "" {
			t.Errorf("member %d decoded without a role", i)
		}
	}
	t.Logf("decoded %d business-group member(s)", len(members))
}

// TestLiveBusinessFilterEncoding is fact B: the business-scoped family
// spells filters as bare field=value, and the accounting family's
// search[field]=value spelling is silently ignored there, not rejected.
//
// The account carries no time entries, so a filter cannot be proved by
// counting results. It is proved by the validator instead: a bare
// updated_since the server cannot parse comes back 422 naming the field,
// while the same bad value under search[updated_since] comes back 200 --
// which can only happen if the server never saw it.
func TestLiveBusinessFilterEncoding(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := liveCtx(t)
	defer cancel()
	m := liveScope(t, c, ctx)

	t.Run("bare field=value reaches the server's validator", func(t *testing.T) {
		opts := &freshbooks.TimeEntryListOptions{Search: freshbooks.Search{"updated_since": "notadate"}}
		_, err := c.TimeEntries.List(ctx, m.BusinessID, opts)
		if err == nil {
			t.Fatal("an unparseable updated_since was accepted; the filter never reached the server")
		}
		if !strings.Contains(err.Error(), "updated_since") {
			t.Errorf("the 422 does not name the field: %v", err)
		}
	})

	t.Run("the accounting search[field] spelling is ignored here", func(t *testing.T) {
		path := fmt.Sprintf("/timetracking/business/%d/time_entries?search[updated_since]=notadate", m.BusinessID)
		var out json.RawMessage
		if err := c.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
			t.Fatalf("the bracket spelling was rejected rather than ignored: %v", err)
		}
	})

	t.Run("a well-formed bare filter is accepted", func(t *testing.T) {
		opts := &freshbooks.TimeEntryListOptions{
			Search: freshbooks.Search{"updated_since": time.Now().Add(-24 * time.Hour).UTC().Format(freshbooks.RFC3339Layout)},
		}
		if _, err := c.TimeEntries.List(ctx, m.BusinessID, opts); err != nil {
			t.Fatalf("TimeEntries.List with updated_since: %v", err)
		}
	})
}

// TestLiveBusinessSortEcho is fact C and fact O together.
//
// C (sort direction) is NOT resolved by this test, and the negative result
// is the finding: the API accepts both the documented "-field" spelling and
// the library's "field_desc" spelling with 200, and echoes back verbatim
// whatever it was given -- including a field name that does not exist. It
// validates nothing, so no probe distinguishes the two spellings on an
// account with zero projects. Sort() is left as it is until an account with
// projects can compare orderings.
//
// O (PageMeta dropping meta.sort) IS resolved: the block is there and the
// library now surfaces it.
func TestLiveBusinessSortEcho(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := liveCtx(t)
	defer cancel()
	m := liveScope(t, c, ctx)

	for _, sort := range []string{"-updated_at", "updated_at_desc", "no_such_field_desc"} {
		path := fmt.Sprintf("/projects/business/%d/projects?per_page=3&sort=%s", m.BusinessID, url.QueryEscape(sort))
		var out struct {
			Meta struct {
				Sort []string `json:"sort"`
			} `json:"meta"`
		}
		if err := c.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
			t.Fatalf("sort=%s was rejected: %v", sort, err)
		}
		if len(out.Meta.Sort) != 1 || out.Meta.Sort[0] != sort {
			t.Errorf("sort=%s echoed back as %v; the echo is meant to be verbatim", sort, out.Meta.Sort)
		}
	}

	// Fact O: the typed list surfaces meta.sort instead of dropping it.
	page, err := c.Projects.List(ctx, m.BusinessID, nil, freshbooks.Sort("updated_at", freshbooks.SortDesc))
	if err != nil {
		t.Fatalf("Projects.List: %v", err)
	}
	if len(page.Sort) == 0 {
		t.Fatal("Page.Sort is empty; the business family's meta.sort is being dropped again")
	}
	t.Logf("the server echoed sort=%v for the library's Sort() encoding", page.Sort)
}

// TestLiveCallbacksEnvelope is fact D: /events/ was classified into the
// accounting family from a Postman example alone. If the classification
// were wrong the transport would look for a {"response":{"result":...}}
// wrapper that is not there, and every pagination field would decode zero.
func TestLiveCallbacksEnvelope(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := liveCtx(t)
	defer cancel()
	m := liveScope(t, c, ctx)

	const perPage = 3
	page, err := c.Callbacks.List(ctx, m.AccountID, nil, freshbooks.PerPage(perPage))
	if err != nil {
		t.Fatalf("Callbacks.List: %v", err)
	}
	// page/per_page live inside the accounting result envelope, so they
	// only survive if the envelope was peeled.
	if page.Page != 1 {
		t.Errorf("page = %d, want 1 -- the accounting envelope was not peeled", page.Page)
	}
	if page.PerPage != perPage {
		t.Errorf("per_page = %d, want %d -- the accounting envelope was not peeled", page.PerPage, perPage)
	}
	if page.Total != len(page.Items) && page.Pages <= 1 {
		t.Errorf("total = %d but the single page holds %d item(s)", page.Total, len(page.Items))
	}
}

// TestLiveDateTimeFormats is fact Q: which live producers send which of the
// wire timestamp formats DateTime accepts. Three of the four are confirmed
// here with named producers; the fourth (the zoneless
// "YYYY-MM-DDTHH:MM:SS") has no live producer on this account, because the
// only endpoints whose Postman examples show it -- projects and time
// entries -- hold no records, so it stays INFERRED.
func TestLiveDateTimeFormats(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := liveCtx(t)
	defer cancel()
	m := liveScope(t, c, ctx)

	t.Run("accounting reads send space-separated and date-only", func(t *testing.T) {
		expenses, err := c.Expenses.List(ctx, m.AccountID, nil, freshbooks.PerPage(1))
		if err != nil {
			t.Fatalf("Expenses.List: %v", err)
		}
		if len(expenses.Items) == 0 {
			t.Skip("no expenses on the authorized account")
		}
		e := expenses.Items[0]
		if e.Date.IsZero() {
			t.Error("expense date did not parse")
		}
		if got := e.Updated.Layout(); got != freshbooks.DateTimeLayout {
			t.Errorf("expense updated parsed as %q, want the space-separated layout", got)
		}

		clients, err := c.Clients.List(ctx, m.AccountID, nil, freshbooks.PerPage(1))
		if err != nil {
			t.Fatalf("Clients.List: %v", err)
		}
		if len(clients.Items) == 0 {
			t.Skip("no clients on the authorized account")
		}
		if got := clients.Items[0].Updated.Layout(); got != freshbooks.DateTimeLayout {
			t.Errorf("client updated parsed as %q, want the space-separated layout", got)
		}
	})

	t.Run("the business and payments families send RFC 3339", func(t *testing.T) {
		accounts, err := c.LedgerAccounts.List(ctx, m.BusinessUUID)
		if err != nil {
			t.Fatalf("LedgerAccounts.List: %v", err)
		}
		if len(accounts) == 0 {
			t.Fatal("the chart of accounts is empty")
		}
		// Ledger accounts send RFC 3339 with fractional seconds, which is
		// why UpdatedAt is a plain time.Time rather than a DateTime.
		if accounts[0].UpdatedAt.IsZero() {
			t.Error("ledger account updated_at did not parse")
		}

		conns, err := c.Gateways.Get(ctx, m.AccountID)
		if err != nil {
			t.Fatalf("Gateways.Get: %v", err)
		}
		for _, conn := range conns {
			if su := conn.StripeUnified; su != nil {
				if got := su.StripeAccountUpdatedAt.Layout(); got != freshbooks.RFC3339Layout {
					t.Errorf("stripe_account_updated_at parsed as %q, want RFC 3339", got)
				}
			}
		}
	})

	t.Run("no live producer for the zoneless layout", func(t *testing.T) {
		projects, err := c.Projects.List(ctx, m.BusinessID, nil, freshbooks.PerPage(1))
		if err != nil {
			t.Fatalf("Projects.List: %v", err)
		}
		if projects.Total != 0 {
			t.Skip("the account now has projects; assert their timestamp layouts and close out fact Q")
		}
		t.Log("the account holds no projects, so the zoneless YYYY-MM-DDTHH:MM:SS layout stays INFERRED from the Postman example")
	})
}
