package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestTasksCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts the task payload", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "tasks", "single")(w, r)
		}))

		task, err := c.Tasks.Create(ctx, AccountID("ACM123"), &TaskCreateRequest{Name: "Saving the planet"})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost || gotPath != "/accounting/account/ACM123/projects/tasks" {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["task"].(map[string]any)
		if inner["name"] != "Saving the planet" {
			t.Fatalf("body = %v", gotBody)
		}
		if task.ID != 74830 || task.Rate.Amount != "50.00" {
			t.Fatalf("task = %+v", task)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Tasks.Create(ctx, AccountID("ACM123"), nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestTasksGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a single task", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "tasks", "single")(w, r)
		}))
		task, err := c.Tasks.Get(ctx, AccountID("ACM123"), 74830)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/projects/tasks/74830" {
			t.Fatalf("path = %q", gotPath)
		}
		if task.Name != "Saving the planet" {
			t.Fatalf("task = %+v", task)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Tasks.Get(ctx, AccountID("ACM123"), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTasksList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a page of tasks", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "tasks", "list"))
		page, err := c.Tasks.List(ctx, AccountID("ACM123"))
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 2 || page.Total != 2 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.Tasks.List(ctx, AccountID("ACM123")); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[happy] All walks a single page and stops", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "tasks", "list"))
		var got []Task
		for task, err := range c.Tasks.All(ctx, AccountID("ACM123")) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, task)
		}
		if len(got) != 2 {
			t.Fatalf("got %d tasks", len(got))
		}
	})
}

func TestTasksUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts the task payload", func(t *testing.T) {
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "tasks", "single")(w, r)
		}))
		name := "Saving the planet"
		task, err := c.Tasks.Update(ctx, AccountID("ACM123"), 74830, &TaskUpdateRequest{Name: &name})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q", gotMethod)
		}
		if task.ID != 74830 {
			t.Fatalf("task = %+v", task)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Tasks.Update(ctx, AccountID("ACM123"), 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestTasksDelete(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] PUTs vis_state 1", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "tasks", "deleted")(w, r)
		}))
		if err := c.Tasks.Delete(ctx, AccountID("ACM123"), 74830); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q", gotMethod)
		}
		inner, _ := gotBody["task"].(map[string]any)
		if inner["vis_state"] != float64(VisStateDeleted) {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a validation error", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if err := c.Tasks.Delete(ctx, AccountID("ACM123"), 1); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}
