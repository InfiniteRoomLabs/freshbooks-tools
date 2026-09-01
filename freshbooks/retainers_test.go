package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestRetainersList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] decodes the flat list; the response carries no meta block", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "retainers", "list")(w, r)
		}))
		page, err := c.Retainers.List(ctx, BusinessID(8675309), nil)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/comments/business/8675309/retainers" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(page.Items) != 2 {
			t.Fatalf("page = %+v", page)
		}
		// The fixture matches FreshBooks' captured example, which has no
		// "meta" block; Total stays zero-valued rather than asserting a
		// pagination fact no evidence supports (see List's doc comment).
		if page.Total != 0 {
			t.Fatalf("page.Total = %d, want 0 (no meta block in the response)", page.Total)
		}
	})

	t.Run("[happy] a business-family search filter is a bare field=value, not search[field]", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "retainers", "list")(w, r)
		}))
		if _, err := c.Retainers.List(ctx, BusinessID(1), &RetainerListOptions{Search: Search{"active": "true"}}); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "active=true" {
			t.Fatalf("query = %q, want the bare business-family encoding", gotQuery)
		}
	})

	t.Run("[sad] a 404 is the business family's bare error string", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if _, err := c.Retainers.List(ctx, BusinessID(1), nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRetainersGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] fetches one retainer", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "retainers", "retainer")(w, r)
		}))
		r, err := c.Retainers.Get(ctx, BusinessID(8675309), 2200)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/comments/business/8675309/retainer/2200" {
			t.Fatalf("path = %q", gotPath)
		}
		if r.ClientUserID != 55001 || r.Fee != "600.00" {
			t.Fatalf("retainer = %+v", r)
		}
	})

	t.Run("[sad] a 404 propagates as ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if _, err := c.Retainers.Get(ctx, BusinessID(1), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRetainersCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the retainer terms", func(t *testing.T) {
		var gotMethod, gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "retainers", "retainer")(w, r)
		}))
		req := &RetainerCreateRequest{ClientUserID: "55001", StartDate: "2026-08-05", Fee: json.Number("600"), ExcessRate: json.Number("200"), Active: true, Frequency: "m"}
		got, err := c.Retainers.Create(ctx, BusinessID(8675309), req)
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/comments/business/8675309/retainers" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		envelope := gotBody["retainer"].(map[string]any)
		if envelope["client_user_id"] != "55001" {
			t.Fatalf("body = %v", gotBody)
		}
		if got.ID != 2200 {
			t.Fatalf("retainer = %+v", got)
		}
	})

	t.Run("[sad] a retainer already exists for the client", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errno": 2001, "error": "Retainer for client already exists"}`))
		}))
		if _, err := c.Retainers.Create(ctx, BusinessID(1), &RetainerCreateRequest{ClientUserID: "1"}); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Retainers.Create(ctx, BusinessID(1), nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestRetainersUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the new terms", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "retainers", "retainer")(w, r)
		}))
		active := true
		req := &RetainerUpdateRequest{Fee: json.Number("20"), ExcessRate: json.Number("700"), Active: &active}
		if _, err := c.Retainers.Update(ctx, BusinessID(8675309), 2200, req); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %s", gotMethod)
		}
		envelope := gotBody["retainer"].(map[string]any)
		if envelope["active"] != true {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[edge] a partial update (Active unset) does not carry active: false", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "retainers", "retainer")(w, r)
		}))
		req := &RetainerUpdateRequest{Fee: json.Number("750")}
		if _, err := c.Retainers.Update(ctx, BusinessID(8675309), 2200, req); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["retainer"].(map[string]any)
		if _, ok := envelope["active"]; ok {
			t.Fatalf("body = %v, want no \"active\" key on a partial update", gotBody)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Retainers.Update(ctx, BusinessID(1), 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestRetainersDeleteUndelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] Delete puts active: false", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "retainers", "retainer")(w, r)
		}))
		if _, err := c.Retainers.Delete(ctx, BusinessID(8675309), 2200); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["retainer"].(map[string]any)
		if envelope["active"] != false {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[happy] Undelete puts active: true", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "retainers", "retainer")(w, r)
		}))
		if _, err := c.Retainers.Undelete(ctx, BusinessID(8675309), 2200); err != nil {
			t.Fatal(err)
		}
		envelope := gotBody["retainer"].(map[string]any)
		if envelope["active"] != true {
			t.Fatalf("body = %v", gotBody)
		}
	})
}
