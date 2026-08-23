//go:build live

// The live suite makes read-only calls against a real FreshBooks account. It
// is never run in CI: it needs a real token and a real account, and it is
// gated behind both the "live" build tag and FRESHBOOKS_LIVE=1.
package freshbooks_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// liveClient builds a client from the environment, or stops the suite.
//
// The skip below is the documented opt-in gate from design spec 8.1, not a
// disabled test hiding a failure, so it carries no issue link: the suite
// only compiles under -tags live in the first place.
func liveClient(t *testing.T) *freshbooks.Client {
	t.Helper()
	if os.Getenv("FRESHBOOKS_LIVE") != "1" {
		t.Skip("set FRESHBOOKS_LIVE=1 to run the read-only live smoke suite")
	}

	var source auth.TokenSource
	switch {
	case os.Getenv("FRESHBOOKS_ACCESS_TOKEN") != "":
		source = auth.StaticTokenSource(os.Getenv("FRESHBOOKS_ACCESS_TOKEN"))
	default:
		path, err := auth.DefaultTokenPath()
		if err != nil {
			t.Fatalf("locating the token file: %v", err)
		}
		source = auth.NewTokenSource(auth.Config{
			ClientID:     os.Getenv("FRESHBOOKS_CLIENT_ID"),
			ClientSecret: os.Getenv("FRESHBOOKS_CLIENT_SECRET"),
		}, auth.NewFileStore(path))
	}

	c, err := freshbooks.NewClient(
		freshbooks.WithTokenSource(source),
		freshbooks.WithUserAgent("freshbooks-go-live-smoke/"+freshbooks.Version),
	)
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}
	return c
}

// TestLiveIdentity is the whole live suite for Phase 1: one read-only call
// that proves auth, the transport, and the auth-family envelope all work
// against the real API.
func TestLiveIdentity(t *testing.T) {
	c := liveClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	memberships, err := c.Identity.Me(ctx)
	if err != nil {
		t.Fatalf("Identity.Me: %v", err)
	}
	if len(memberships) == 0 {
		t.Fatal("the account has no business memberships")
	}
	for i, m := range memberships {
		// Identifiers are account data; log only their presence.
		if m.AccountID == "" {
			t.Errorf("membership %d has no account_id", i)
		}
		if m.BusinessID == 0 {
			t.Errorf("membership %d has no business id", i)
		}
	}
	t.Logf("live identity check passed with %d membership(s)", len(memberships))
}
