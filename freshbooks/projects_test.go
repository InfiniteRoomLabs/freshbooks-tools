package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
)

func TestProjectsCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the project payload", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "projects", "single")(w, r)
		}))

		p, err := c.Projects.Create(ctx, BusinessID(8675309), &ProjectCreateRequest{
			Title: "Very Custom Computer Thing", ClientID: 55001, ProjectType: "fixed_price",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/projects/business/8675309/project" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["project"].(map[string]any)
		if inner["title"] != "Very Custom Computer Thing" {
			t.Fatalf("body = %v", gotBody)
		}
		if p.ID != 2976412 {
			t.Fatalf("project = %+v", p)
		}
		if p.CreatedAt.IsZero() {
			t.Fatal("CreatedAt did not parse the zoneless timestamp")
		}
		if p.Description != "" {
			t.Fatalf("a captured-null description should decode to empty: %+v", p)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Projects.Create(ctx, BusinessID(1), nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a validation error", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"error": "invalid_input", "error_description": "title is required"}`)
		}))
		_, err := c.Projects.Create(ctx, BusinessID(1), &ProjectCreateRequest{Title: "x"})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProjectsGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a single project, its sibling abilities dropped", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "projects", "single")(w, r)
		}))
		p, err := c.Projects.Get(ctx, BusinessID(8675309), 2976412)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/projects/business/8675309/projects/2976412" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(p.Services) != 1 || p.Services[0].Name != "Computer Hardware" {
			t.Fatalf("services = %+v", p.Services)
		}
		if p.DueDate.IsZero() != true {
			t.Errorf("a null due_date should decode to zero")
		}
		if p.GroupID != 0 {
			t.Errorf("Get returns the expanded Group, not GroupID: %+v", p)
		}
		if len(p.Group) == 0 {
			t.Error("Group should carry the raw expanded object")
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if _, err := c.Projects.Get(ctx, BusinessID(1), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProjectsList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a page of projects, carrying group_id", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "projects", "list_full"))
		page, err := c.Projects.List(ctx, BusinessID(8675309), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Total != 1 {
			t.Fatalf("page = %+v", page)
		}
		if page.Items[0].GroupID != 4951020 {
			t.Fatalf("GroupID = %d", page.Items[0].GroupID)
		}
	})

	t.Run("[happy] All walks a single page and stops", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "projects", "list_full"))
		var got []Project
		for p, err := range c.Projects.All(ctx, BusinessID(8675309), nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, p)
		}
		if len(got) != 1 {
			t.Fatalf("got %d projects", len(got))
		}
	})

	t.Run("[happy] Search filters spell as bare field=value", func(t *testing.T) {
		var gotQuery string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			serveFixture(t, http.StatusOK, "projects", "list_full")(w, r)
		}))
		opts := &ProjectListOptions{Search: Search{"active": "true"}}
		if _, err := c.Projects.List(ctx, BusinessID(8675309), opts); err != nil {
			t.Fatal(err)
		}
		if gotQuery != "active=true" {
			t.Fatalf("query = %q", gotQuery)
		}
	})

	t.Run("[corner] a PageNumber passed through extra cannot pin All to one page", func(t *testing.T) {
		var gotPages []string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageStr := r.URL.Query().Get("page")
			gotPages = append(gotPages, pageStr)
			n, err := strconv.Atoi(pageStr)
			if err != nil {
				n = 1
			}
			resp := struct {
				Meta struct {
					Page    int `json:"page"`
					Pages   int `json:"pages"`
					PerPage int `json:"per_page"`
					Total   int `json:"total"`
				} `json:"meta"`
				Projects []Project `json:"projects"`
			}{}
			resp.Meta.Page, resp.Meta.Pages, resp.Meta.PerPage, resp.Meta.Total = n, 3, 1, 3
			resp.Projects = []Project{{ID: int64(n), Title: fmt.Sprintf("p%d", n)}}
			body, _ := json.Marshal(resp)
			_, _ = w.Write(body)
		}))

		var gotIDs []int64
		for p, err := range c.Projects.All(ctx, BusinessID(8675309), nil, PageNumber(3)) {
			if err != nil {
				t.Fatal(err)
			}
			gotIDs = append(gotIDs, p.ID)
		}
		if want := []string{"1", "2", "3"}; !equalStrings(gotPages, want) {
			t.Fatalf("pages requested = %v, want %v (extra's PageNumber(3) must not pin the walk)", gotPages, want)
		}
		if want := []int64{1, 2, 3}; !equalInt64s(gotIDs, want) {
			t.Fatalf("ids = %v, want %v", gotIDs, want)
		}
	})

	t.Run("[sad] a 429 is ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"), WithRetry(NoRetry))
		if _, err := c.Projects.List(ctx, BusinessID(1), nil); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProjectsUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the project payload", func(t *testing.T) {
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "projects", "single")(w, r)
		}))
		title := "Renamed"
		p, err := c.Projects.Update(ctx, BusinessID(8675309), 2976412, &ProjectUpdateRequest{Title: &title})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q", gotMethod)
		}
		if p.ID != 2976412 {
			t.Fatalf("project = %+v", p)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Projects.Update(ctx, BusinessID(1), 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestProjectsDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] DELETEs the documented public path with no body", func(t *testing.T) {
		// No captured success example exists for this endpoint on either
		// path (the Postman my.freshbooks.com capture is itself a 404; the
		// docs page carries no example body): the 204 this handler returns
		// is invented, not captured. Delete decodes nothing, so a bare
		// status is all that's needed either way.
		var gotMethod, gotPath string
		var gotBody []byte
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod, gotPath = r.Method, r.URL.Path
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		}))
		if err := c.Projects.Delete(ctx, BusinessID(8675309), 2976412); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodDelete || gotPath != "/projects/business/8675309/project/2976412" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if len(gotBody) != 0 {
			t.Fatalf("body = %q, want no body per the documented endpoint", gotBody)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		err := c.Projects.Delete(ctx, BusinessID(1), 1)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProjectsAbilities(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] lists permission flags", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "projects", "abilities")(w, r)
		}))
		abilities, err := c.Projects.Abilities(ctx, BusinessID(8675309))
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/projects/business/8675309/abilities" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(abilities) != 3 || !abilities[0].Value {
			t.Fatalf("abilities = %+v", abilities)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Projects.Abilities(ctx, BusinessID(1)); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestProjectsDiscussions(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] Threads returns the undecoded list", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "projects", "threads")(w, r)
		}))
		threads, err := c.Projects.Threads(ctx, BusinessID(8675309), 2976412)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/comments/business/8675309/project/2976412/threads" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(threads) != 1 || threads[0]["message"] != "This project will be amazing!" {
			t.Fatalf("threads = %v", threads)
		}
	})

	t.Run("[happy] CreateThread posts the opening message", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "projects", "thread_created")(w, r)
		}))
		resp, err := c.Projects.CreateThread(ctx, BusinessID(8675309), 2976412, "This project will be amazing!")
		if err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["thread"].(map[string]any)
		if inner["message"] != "This project will be amazing!" {
			t.Fatalf("body = %v", gotBody)
		}
		if resp["id"] != float64(991002) {
			t.Fatalf("resp = %v", resp)
		}
	})

	t.Run("[happy] AddThreadComment posts a reply", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "projects", "comment_added")(w, r)
		}))
		resp, err := c.Projects.AddThreadComment(ctx, BusinessID(8675309), 991001, "Sounds like a plan!")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/comments/business/8675309/threads/991001/comments" {
			t.Fatalf("path = %q", gotPath)
		}
		inner, _ := gotBody["comment"].(map[string]any)
		if inner["content"] != "Sounds like a plan!" {
			t.Fatalf("body = %v", gotBody)
		}
		if resp["content"] != "Sounds like a plan!" {
			t.Fatalf("resp = %v", resp)
		}
	})

	t.Run("[sad] a 404 on AddThreadComment", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if _, err := c.Projects.AddThreadComment(ctx, BusinessID(1), 1, "x"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}
