package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestTeamMembersList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] the response array and meta decode into a Page", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "team_members", "list")(w, r)
		}))

		page, err := c.TeamMembers.List(ctx, BusinessID(8675309))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/auth/api/v1/businesses/8675309/team_members" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(page.Items) != 1 || page.Total != 1 {
			t.Fatalf("page = %+v", page)
		}
		if page.Items[0].UUID != "00000000-0000-4000-8000-000000000101" {
			t.Errorf("member = %+v", page.Items[0])
		}
	})

	t.Run("[happy] All walks a single page and stops", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "team_members", "list"))
		var got []TeamMember
		for m, err := range c.TeamMembers.All(ctx, BusinessID(8675309)) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, m)
		}
		if len(got) != 1 {
			t.Fatalf("got %d members", len(got))
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.TeamMembers.List(ctx, BusinessID(1)); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTeamMembersGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a single member", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "team_members", "single")(w, r)
		}))
		m, err := c.TeamMembers.Get(ctx, BusinessID(8675309), "00000000-0000-4000-8000-000000000102")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/auth/api/v1/businesses/8675309/team_members/00000000-0000-4000-8000-000000000102" {
			t.Fatalf("path = %q", gotPath)
		}
		if m.Email != "riley@example.com" || m.Country != "United States" {
			t.Fatalf("member = %+v", m)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if _, err := c.TeamMembers.Get(ctx, BusinessID(1), "missing"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTeamMembersInvitationRates(t *testing.T) {
	ctx := context.Background()
	t.Run("[happy] lists rates offered to invitees", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "team_members", "invitation_rates"))
		rates, err := c.TeamMembers.InvitationRates(ctx, BusinessID(8675309))
		if err != nil {
			t.Fatal(err)
		}
		if len(rates) != 1 || rates[0].Rate != "45.00" {
			t.Fatalf("rates = %+v", rates)
		}
	})

	t.Run("[edge] an empty list", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"invitation_rates": []}`)
		}))
		rates, err := c.TeamMembers.InvitationRates(ctx, BusinessID(1))
		if err != nil {
			t.Fatal(err)
		}
		if len(rates) != 0 {
			t.Fatalf("rates = %+v", rates)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.TeamMembers.InvitationRates(ctx, BusinessID(1)); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTeamMembersRates(t *testing.T) {
	ctx := context.Background()
	t.Run("[happy] lists every member's rate", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "team_members", "rates"))
		rates, err := c.TeamMembers.Rates(ctx, BusinessID(8675309))
		if err != nil {
			t.Fatal(err)
		}
		if len(rates) != 2 {
			t.Fatalf("rates = %+v", rates)
		}
	})

	t.Run("[sad] a 429 is ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"), WithRetry(NoRetry))
		if _, err := c.TeamMembers.Rates(ctx, BusinessID(1)); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTeamMembersUpdateRate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts identity_id and rate", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "team_members", "rate_updated")(w, r)
		}))

		rate, err := c.TeamMembers.UpdateRate(ctx, BusinessID(8675309), 4242424, "40")
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut || gotPath != "/comments/business/8675309/team_member_rate/4242424" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["team_member_rate"].(map[string]any)
		if inner["rate"] != "40" || inner["identity_id"] != float64(4242424) {
			t.Fatalf("body = %v", gotBody)
		}
		if rate.Rate != "40.00" {
			t.Fatalf("rate = %+v", rate)
		}
	})

	t.Run("[sad] a validation error", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if _, err := c.TeamMembers.UpdateRate(ctx, BusinessID(1), 1, "40"); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTeamMembersInvite(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the invitation payload", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "team_members", "invited")(w, r)
		}))

		resp, err := c.TeamMembers.Invite(ctx, &InviteRequest{
			Capacity:      "manager",
			ToEmail:       "invitee@example.com",
			InvitableID:   8675309,
			InvitableType: "business",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/auth/api/v1/users/invitation" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotBody["to_email"] != "invitee@example.com" {
			t.Fatalf("body = %v", gotBody)
		}
		if resp["to_email"] != "invitee@example.com" {
			t.Fatalf("resp = %v", resp)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.TeamMembers.Invite(ctx, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}
