package freshbooks

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestExpenseCategoriesList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of categories", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "expense_categories_list"))
		page, err := c.ExpenseCategories.List(ctx, acct, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].Category != "Advertising" {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 429 is ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"))
		if _, err := c.ExpenseCategories.List(ctx, acct, nil); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExpenseCategoriesAll(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] auto-paginates until a short page", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "expense_categories_list"))
		var got []ExpenseCategory
		for cat, err := range c.ExpenseCategories.All(ctx, acct, nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, cat)
		}
		if len(got) != 1 {
			t.Fatalf("got %d categories", len(got))
		}
	})
}

func TestExpenseCategoriesGet(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] fetches by id", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "accounting", "expense_categories_get")(w, r)
		}))
		cat, err := c.ExpenseCategories.Get(ctx, acct, 2003192)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/expenses/categories/2003192" {
			t.Fatalf("path = %q", gotPath)
		}
		if cat.CategoryID != 2003192 {
			t.Fatalf("category = %+v", cat)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.ExpenseCategories.Get(ctx, acct, 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestExpenseCategoriesCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the category payload", func(t *testing.T) {
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "accounting", "expense_categories_create")(w, r)
		}))
		cat, err := c.ExpenseCategories.Create(ctx, acct, &ExpenseCategoryCreateRequest{Category: "Custom Category"})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q", gotMethod)
		}
		if cat.Category != "Custom Category" || !cat.IsEditable {
			t.Fatalf("category = %+v", cat)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.ExpenseCategories.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestExpenseCategoriesRejectUnsafeAccountID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		acct AccountID
	}{
		{"[sad] a path separator", "a/b"},
		{"[sad] a query delimiter", "a?b"},
		{"[sad] a fragment delimiter", "a#b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			if _, err := c.ExpenseCategories.Get(ctx, tc.acct, 1); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
