package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestStaffList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] unwraps business_group.members", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "staff", "list")(w, r)
		}))

		members, err := c.Staff.List(ctx, BusinessID(8675309))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/auth/api/v1/users/business/8675309" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(members) != 2 {
			t.Fatalf("got %d members", len(members))
		}
		if members[0].IdentityID != 4242424 || members[0].Role != "owner" {
			t.Errorf("member = %+v", members[0])
		}
	})

	t.Run("[edge] no members decodes to an empty slice", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"response": {"business_group": {"members": []}}}`)
		}))
		members, err := c.Staff.List(ctx, BusinessID(1))
		if err != nil {
			t.Fatal(err)
		}
		if len(members) != 0 {
			t.Fatalf("got %d members", len(members))
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Staff.List(ctx, BusinessID(1)); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestStaffGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes the enveloped staff record", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "staff", "single")(w, r)
		}))

		staff, err := c.Staff.Get(ctx, AccountID("ACM123"), 1)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/users/staffs/1" {
			t.Fatalf("path = %q", gotPath)
		}
		if staff.FirstName != "Ryen" || staff.Role != "admin" || staff.VisState != VisStateActive {
			t.Fatalf("staff = %+v", staff)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Staff.Get(ctx, AccountID("ACM123"), 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestStaffUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the staff payload", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "staff", "single")(w, r)
		}))

		fname := "Ryen"
		staff, err := c.Staff.Update(ctx, AccountID("ACM123"), 1, &StaffUpdateRequest{FirstName: &fname})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q", gotMethod)
		}
		inner, _ := gotBody["staff"].(map[string]any)
		if inner["fname"] != "Ryen" {
			t.Fatalf("body = %v", gotBody)
		}
		if staff.ID != 1 {
			t.Fatalf("staff = %+v", staff)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Staff.Update(ctx, AccountID("ACM123"), 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a 403 is ErrForbidden", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusForbidden, "staff", "error_403"))
		if _, err := c.Staff.Update(ctx, AccountID("ACM123"), 1, &StaffUpdateRequest{}); !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestStaffDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] PUTs vis_state 1, the only delete verb this family has", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "staff", "deleted")(w, r)
		}))

		if err := c.Staff.Delete(ctx, AccountID("ACM123"), 1); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q", gotMethod)
		}
		inner, _ := gotBody["staff"].(map[string]any)
		if inner["vis_state"] != float64(VisStateDeleted) {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a 403 is ErrForbidden", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusForbidden, "staff", "error_403"))
		if err := c.Staff.Delete(ctx, AccountID("ACM123"), 1); !errors.Is(err, ErrForbidden) {
			t.Fatalf("err = %v", err)
		}
	})
}
