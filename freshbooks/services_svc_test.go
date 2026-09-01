package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestServicesGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] the business-family service record", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "services", "single")(w, r)
		}))
		svc, err := c.Services.Get(ctx, BusinessID(8675309), 4054453)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/comments/business/8675309/service/4054453" {
			t.Fatalf("path = %q", gotPath)
		}
		if svc.Name != "Document Review" || !svc.Billable {
			t.Fatalf("service = %+v", svc)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if _, err := c.Services.Get(ctx, BusinessID(1), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestServicesList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a page of services", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "services", "list"))
		page, err := c.Services.List(ctx, BusinessID(8675309), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 2 || page.Total != 2 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[happy] a Search option filters as bare field=value", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "services", "list")(w, r)
		}))
		opts := &ServiceListOptions{Search: Search{"billable": "true"}}
		if _, err := c.Services.List(ctx, BusinessID(8675309), opts); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "billable=true" {
			t.Fatalf("query = %q", gotQuery)
		}
	})

	t.Run("[edge] an empty list", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"services": [], "meta": {"total": 0, "page": 1, "pages": 0, "per_page": 50}}`)
		}))
		page, err := c.Services.List(ctx, BusinessID(1), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 0 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Services.List(ctx, BusinessID(1), nil); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestServicesCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a billable_item to the accounting family", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "services", "billable_item")(w, r)
		}))

		item, err := c.Services.Create(ctx, AccountID("ACM123"), &BillableItemCreateRequest{
			Name: "Working hard", Billable: true, Description: "Cool dude",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/accounting/account/ACM123/billable_items/billable_items" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["billable_item"].(map[string]any)
		if inner["name"] != "Working hard" {
			t.Fatalf("body = %v", gotBody)
		}
		if item.ID != 60001 || item.UnitCost.Amount != "50.00" {
			t.Fatalf("item = %+v", item)
		}
		if item.Updated.IsZero() {
			t.Fatal("Updated did not parse")
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Services.Create(ctx, AccountID("ACM123"), nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] an accounting validation error", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if _, err := c.Services.Create(ctx, AccountID("ACM123"), &BillableItemCreateRequest{Name: "x"}); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] a hostile AccountID never reaches the network", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Services.Create(ctx, AccountID("../x"), &BillableItemCreateRequest{Name: "x"}); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestServicesGetBillableItem(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] the accounting-family billable item", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "services", "billable_item")(w, r)
		}))
		item, err := c.Services.GetBillableItem(ctx, AccountID("ACM123"), 60001)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/billable_items/60001" {
			t.Fatalf("path = %q", gotPath)
		}
		if item.Name != "Working hard" {
			t.Fatalf("item = %+v", item)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Services.GetBillableItem(ctx, AccountID("ACM123"), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] a hostile AccountID never reaches the network", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Services.GetBillableItem(ctx, AccountID("a?b"), 1); err == nil {
			t.Fatal("want an error")
		}
	})
}
