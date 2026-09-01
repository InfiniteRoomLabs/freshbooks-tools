package freshbooks

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestSystemsGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes the enveloped system record", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "systems", "get")(w, r)
		}))

		sys, err := c.Systems.Get(ctx, AccountID("ACM123"), BusinessID(4236410))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/systems/systems/4236410" {
			t.Fatalf("path = %q", gotPath)
		}
		if sys.ID != 4236410 || sys.CurrencyCode != "USD" || sys.Country != "Canada" {
			t.Fatalf("system = %+v", sys)
		}
		if sys.BusinessUUID != "" {
			t.Fatalf("a null business_uuid should decode to empty, got %q", sys.BusinessUUID)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Systems.Get(ctx, AccountID("ACM123"), BusinessID(1)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}
