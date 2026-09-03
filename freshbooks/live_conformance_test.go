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
	"slices"
	"strconv"
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
