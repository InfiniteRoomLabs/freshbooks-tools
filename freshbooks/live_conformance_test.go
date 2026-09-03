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
