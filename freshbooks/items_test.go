package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestItemsList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes the page", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "items", "list")(w, r)
		}))
		page, err := c.Items.List(ctx, "ACM123", nil)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/items/items" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(page.Items) != 2 || page.Total != 2 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[happy] filtering by SKU is List with a Search option", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "items", "list")(w, r)
		}))
		if _, err := c.Items.List(ctx, "ACM123", &ItemListOptions{Search: Search{"sku": "KS994RUR"}}); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "search%5Bsku%5D=KS994RUR" {
			t.Fatalf("query = %q", gotQuery)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Items.List(ctx, "ACM123", nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestItemsAll(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] iterates every item once", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "items", "list"))
		var got []Item
		for it, err := range c.Items.All(ctx, "ACM123", nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, it)
		}
		if len(got) != 2 {
			t.Fatalf("got %d items", len(got))
		}
	})
}

func TestItemsGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] fetches one item", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "items", "item")(w, r)
		}))
		it, err := c.Items.Get(ctx, "ACM123", 40001)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/items/items/40001" {
			t.Fatalf("path = %q", gotPath)
		}
		if it.Name != "Trail Pack" || it.SKU != "KS994RUR" {
			t.Fatalf("item = %+v", it)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Items.Get(ctx, "ACM123", 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestItemsCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a new catalogue item", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "items", "item")(w, r)
		}))
		req := &ItemCreateRequest{Name: "Trail Pack", SKU: "KS994RUR", Qty: "10", Inventory: "50", UnitCost: Money{Amount: "550.00", Code: "CAD"}}
		it, err := c.Items.Create(ctx, "ACM123", req)
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/accounting/account/ACM123/items/items" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		envelope := gotBody["item"].(map[string]any)
		if envelope["sku"] != "KS994RUR" {
			t.Fatalf("body = %v", gotBody)
		}
		if it.ItemID != 40001 {
			t.Fatalf("item = %+v", it)
		}
	})

	t.Run("[edge] an unset UnitCost is omitted, not sent as an empty Money", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "items", "item")(w, r)
		}))
		if _, err := c.Items.Create(ctx, "ACM123", &ItemCreateRequest{Name: "Free Sample"}); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["item"].(map[string]any)
		if _, ok := envelope["unit_cost"]; ok {
			t.Fatalf("body = %v, want an unset UnitCost omitted", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Items.Create(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestItemsUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "items", "item")(w, r)
		}))
		inv := "4"
		if _, err := c.Items.Update(ctx, "ACM123", 40001, &ItemUpdateRequest{Inventory: &inv}); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %s", gotMethod)
		}
		envelope := gotBody["item"].(map[string]any)
		if envelope["inventory"] != "4" {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Items.Update(ctx, "ACM123", 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestItemsDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts vis_state 1", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "items", "item")(w, r)
		}))
		if err := c.Items.Delete(ctx, "ACM123", 40001); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["item"].(map[string]any)
		if envelope["vis_state"] != float64(1) {
			t.Fatalf("body = %v", gotBody)
		}
	})
}
