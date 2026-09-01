package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestTimeEntriesList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a page of entries", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "time_entries", "list")(w, r)
		}))
		page, err := c.TimeEntries.List(ctx, BusinessID(8675309), nil)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/timetracking/business/8675309/time_entries" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(page.Items) != 2 || page.Total != 2 {
			t.Fatalf("page = %+v", page)
		}
		if page.Items[0].StartedAt.IsZero() {
			t.Fatal("StartedAt did not parse")
		}
		if page.Items[0].PendingClient != nil || page.Items[0].PendingProject != nil || page.Items[0].PendingTask != nil {
			t.Fatalf("a captured-null field should decode to a nil pointer: %+v", page.Items[0])
		}
	})

	t.Run("[happy] filters spell as bare field=value, not search[field]", func(t *testing.T) {
		var gotQuery url.Values
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			serveFixture(t, http.StatusOK, "time_entries", "list")(w, r)
		}))
		opts := &TimeEntryListOptions{Search: Search{"updated_since": "2026-08-01T03:00:00Z", "include_deleted": "1"}}
		_, err := c.TimeEntries.List(ctx, BusinessID(8675309), opts)
		if err != nil {
			t.Fatal(err)
		}
		if gotQuery.Get("updated_since") != "2026-08-01T03:00:00Z" || gotQuery.Get("include_deleted") != "1" {
			t.Fatalf("query = %v", gotQuery)
		}
		if gotQuery.Get("search[updated_since]") != "" {
			t.Fatal("business family must not use search[field]")
		}
	})

	t.Run("[edge] an empty page", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"meta": {"total": 0, "page": 1, "pages": 0, "per_page": 15}, "time_entries": []}`)
		}))
		page, err := c.TimeEntries.List(ctx, BusinessID(1), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 0 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.TimeEntries.List(ctx, BusinessID(1), nil); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTimeEntriesSearch(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] hits the search endpoint with q and useSearchEndpoint", func(t *testing.T) {
		var gotPath string
		var gotQuery url.Values
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotQuery = r.URL.Path, r.URL.Query()
			serveFixture(t, http.StatusOK, "time_entries", "list")(w, r)
		}))
		page, err := c.TimeEntries.Search(ctx, BusinessID(8675309), "Jordan", nil, Sort("started_at", SortDesc))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/timetracking/business/8675309/time_entries/search" {
			t.Fatalf("path = %q", gotPath)
		}
		if gotQuery.Get("q") != "Jordan" || gotQuery.Get("useSearchEndpoint") != "true" {
			t.Fatalf("query = %v", gotQuery)
		}
		if len(page.Items) != 2 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[happy] merges opts.Search alongside q", func(t *testing.T) {
		var gotQuery url.Values
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.Query()
			serveFixture(t, http.StatusOK, "time_entries", "list")(w, r)
		}))
		opts := &TimeEntryListOptions{Search: Search{"billed": "true"}}
		if _, err := c.TimeEntries.Search(ctx, BusinessID(8675309), "Jordan", opts); err != nil {
			t.Fatal(err)
		}
		if gotQuery.Get("q") != "Jordan" || gotQuery.Get("billed") != "true" {
			t.Fatalf("query = %v", gotQuery)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if _, err := c.TimeEntries.Search(ctx, BusinessID(1), "x", nil); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTimeEntriesCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the entry payload", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "time_entries", "single")(w, r)
		}))

		entry, err := c.TimeEntries.Create(ctx, BusinessID(8675309), &TimeEntryCreateRequest{
			IsLogged:  true,
			Duration:  7200,
			StartedAt: NewDateTime(time.Date(2026, 8, 16, 20, 0, 0, 0, time.UTC)),
			ClientID:  55001,
			Note:      "Stuff",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/timetracking/business/8675309/time_entries" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["time_entry"].(map[string]any)
		if inner["duration"] != float64(7200) || inner["client_id"] != float64(55001) {
			t.Fatalf("body = %v", gotBody)
		}
		if entry.ID != 47902064 {
			t.Fatalf("entry = %+v", entry)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.TimeEntries.Create(ctx, BusinessID(1), nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a validation error (business-family flat shape)", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"error": "invalid_input", "error_description": "duration is required"}`)
		}))
		_, err := c.TimeEntries.Create(ctx, BusinessID(1), &TimeEntryCreateRequest{})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTimeEntriesUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the entry payload", func(t *testing.T) {
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "time_entries", "single")(w, r)
		}))
		note := "Updated"
		entry, err := c.TimeEntries.Update(ctx, BusinessID(8675309), 47902064, &TimeEntryUpdateRequest{Note: &note})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q", gotMethod)
		}
		if entry.ID != 47902064 {
			t.Fatalf("entry = %+v", entry)
		}
	})

	t.Run("[happy] Timer.IsRunning=false actually sends false", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "time_entries", "single")(w, r)
		}))
		_, err := c.TimeEntries.Update(ctx, BusinessID(8675309), 47902064, &TimeEntryUpdateRequest{
			Timer: &Timer{ID: "t1", IsRunning: false},
		})
		if err != nil {
			t.Fatal(err)
		}
		timerBody, _ := gotBody["time_entry"].(map[string]any)
		timer, _ := timerBody["timer"].(map[string]any)
		v, ok := timer["is_running"]
		if !ok || v != false {
			t.Fatalf("is_running should be sent as an explicit false, got %v (present=%v)", v, ok)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.TimeEntries.Update(ctx, BusinessID(1), 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestTimeEntriesDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] issues a real DELETE", func(t *testing.T) {
		var gotMethod, gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		}))
		if err := c.TimeEntries.Delete(ctx, BusinessID(8675309), 47902064); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodDelete || gotPath != "/timetracking/business/8675309/time_entries/47902064" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if err := c.TimeEntries.Delete(ctx, BusinessID(1), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}
