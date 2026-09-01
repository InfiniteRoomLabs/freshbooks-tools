package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestEstimatesList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of estimates", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "estimates_list"))
		page, err := c.Estimates.List(ctx, acct, &EstimateListOptions{Include: []string{"lines"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].CustomerID != 55001 {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 429 is ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"))
		if _, err := c.Estimates.List(ctx, acct, nil); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestEstimatesAll(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] auto-paginates until a short page", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "estimates_list"))
		var got []Estimate
		for e, err := range c.Estimates.All(ctx, acct, nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, e)
		}
		if len(got) != 1 {
			t.Fatalf("got %d estimates", len(got))
		}
	})
}

func TestEstimatesGet(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] fetches by id", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "accounting", "estimates_get")(w, r)
		}))
		est, err := c.Estimates.Get(ctx, acct, 1706)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/estimates/estimates/1706" {
			t.Fatalf("path = %q", gotPath)
		}
		if est.DisplayStatus != "draft" {
			t.Fatalf("estimate = %+v", est)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Estimates.Get(ctx, acct, 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestEstimatesCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the estimate payload", func(t *testing.T) {
		var gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			serveFixture(t, http.StatusOK, "accounting", "estimates_create")(w, r)
		}))
		est, err := c.Estimates.Create(ctx, acct, &EstimateWriteRequest{
			CustomerID: 55001,
			Lines:      []EstimateLine{{Name: "Consulting", Qty: "1", UnitCost: Money{Amount: "500.00", Code: "USD"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q", gotMethod)
		}
		if est.EstimateID != 2652 {
			t.Fatalf("estimate = %+v", est)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Estimates.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestEstimatesUpdate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			serveFixture(t, http.StatusOK, "accounting", "estimates_get")(w, r)
		}))
		if _, err := c.Estimates.Update(ctx, acct, 1706, &EstimateWriteRequest{Notes: "Updated"}); err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/estimates/estimates/1706" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.Estimates.Update(ctx, acct, 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestEstimatesDelete(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] soft-deletes via a vis_state PUT", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"response": {}}`))
		}))
		if err := c.Estimates.Delete(ctx, acct, 1706); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q, want PUT", gotMethod)
		}
		inner, _ := gotBody["estimate"].(map[string]any)
		if inner["vis_state"] != float64(VisStateDeleted) {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a 422 is ErrValidation", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if err := c.Estimates.Delete(ctx, acct, 1706); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestEstimatesAccept(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] sets action_accept", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "estimates_accept")(w, r)
		}))
		est, err := c.Estimates.Accept(ctx, acct, 1706)
		if err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["estimate"].(map[string]any)
		if inner["action_accept"] != true {
			t.Fatalf("body = %v", gotBody)
		}
		if !est.Accepted {
			t.Fatalf("estimate = %+v", est)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		if _, err := c.Estimates.Accept(ctx, acct, 999); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestEstimatesSend(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] sets action_email with recipients", func(t *testing.T) {
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"response": {}}`))
		}))
		err := c.Estimates.Send(ctx, acct, 1706, &EstimateSendRequest{
			EmailRecipients: []string{"client@example.com"},
		})
		if err != nil {
			t.Fatal(err)
		}
		inner, _ := gotBody["estimate"].(map[string]any)
		if inner["action_email"] != true {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] no recipients", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if err := c.Estimates.Send(ctx, acct, 1706, &EstimateSendRequest{}); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if err := c.Estimates.Send(ctx, acct, 1706, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestEstimatesRejectUnsafeAccountID(t *testing.T) {
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
			if _, err := c.Estimates.Get(ctx, tc.acct, 1); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
