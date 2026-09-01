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
		if sys.ID != 4236410 || sys.SystemID != 4236410 {
			t.Fatalf("system = %+v", sys)
		}
		if sys.AccountID != "ACM123" || sys.CurrencyCode != "USD" || sys.Country != "Canada" {
			t.Fatalf("system = %+v", sys)
		}
		if sys.Name != "Example Business LLC" || sys.Email != "owner@example.com" || sys.InfoEmail != sys.Email {
			t.Fatalf("system = %+v", sys)
		}
		if !sys.Active || !sys.DST || sys.TestSystem || !sys.ModernSystem || !sys.MasterlockBilling {
			t.Fatalf("system = %+v", sys)
		}
		if sys.Duration != 12 || sys.AutoBill != 5 || sys.PaymentFrequency != 1 || sys.TimezoneID != 14 {
			t.Fatalf("system = %+v", sys)
		}
		if sys.Timezone != "UTC" || sys.BillingStatus != "uptodate" || sys.ReferralID != "multibiz" {
			t.Fatalf("system = %+v", sys)
		}
		if sys.GSTAmount.Amount != "0.00" || sys.GSTAmount.Code != "USD" {
			t.Fatalf("GSTAmount = %+v", sys.GSTAmount)
		}
		if sys.PaymentAmount.Amount != "0.00" || sys.PaymentAmount.Code != "USD" {
			t.Fatalf("PaymentAmount = %+v", sys.PaymentAmount)
		}
		if sys.Date.IsZero() {
			t.Fatal("Date did not parse")
		}
		if sys.BusinessUUID != nil || sys.BusinessType != nil || sys.Street2 != nil || sys.DiscountID != nil ||
			sys.ReferringURL != nil || sys.LandingURL != nil || sys.HeardAboutUsVia != nil || sys.Salutation != nil ||
			sys.NumClients != nil || sys.NumStaff != nil || sys.SizeLimit != nil || sys.SplitToken != nil ||
			sys.VATName != nil || sys.VATNumber != nil || sys.MigratedToSmuxAt != nil {
			t.Fatalf("a captured-null field should decode to a nil pointer: %+v", sys)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Systems.Get(ctx, AccountID("ACM123"), BusinessID(1)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] a hostile AccountID never reaches the network", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Systems.Get(ctx, AccountID("../x"), BusinessID(1)); err == nil {
			t.Fatal("want an error")
		}
	})
}
