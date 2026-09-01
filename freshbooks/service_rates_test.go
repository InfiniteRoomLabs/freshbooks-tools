package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestServiceRatesGet(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] one service's rate", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "service_rates", "single")(w, r)
		}))
		rate, err := c.ServiceRates.Get(ctx, BusinessID(8675309), 4054453)
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/comments/business/8675309/service/4054453/rate" {
			t.Fatalf("path = %q", gotPath)
		}
		if rate.Rate != "10.00" {
			t.Fatalf("rate = %+v", rate)
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "projects", "error_404"))
		if _, err := c.ServiceRates.Get(ctx, BusinessID(1), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestServiceRatesList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] every service's rate", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "service_rates", "list"))
		rates, err := c.ServiceRates.List(ctx, BusinessID(8675309))
		if err != nil {
			t.Fatal(err)
		}
		if len(rates) != 2 {
			t.Fatalf("rates = %+v", rates)
		}
	})

	t.Run("[edge] an empty list", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"service_rates": []}`)
		}))
		rates, err := c.ServiceRates.List(ctx, BusinessID(1))
		if err != nil {
			t.Fatal(err)
		}
		if len(rates) != 0 {
			t.Fatalf("rates = %+v", rates)
		}
	})

	t.Run("[sad] a 401 is ErrUnauthorized", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnauthorized, "auth", "error_401"))
		if _, err := c.ServiceRates.List(ctx, BusinessID(1)); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestServiceRatesUpdateProjectRate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] puts a project-scoped rate", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "service_rates", "project_rate_updated")(w, r)
		}))

		rate, err := c.ServiceRates.UpdateProjectRate(ctx, BusinessID(8675309), 2991018, 4054454, "1020")
		if err != nil {
			t.Fatal(err)
		}
		wantPath := "/comments/business/8675309/project/2991018/service/4054454/rate"
		if gotMethod != http.MethodPut || gotPath != wantPath {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["project_service_rate"].(map[string]any)
		if inner["rate"] != "1020" {
			t.Fatalf("body = %v", gotBody)
		}
		if rate.Rate != "1020.00" || rate.ProjectID != 2991018 {
			t.Fatalf("rate = %+v", rate)
		}
	})

	t.Run("[sad] a validation error", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if _, err := c.ServiceRates.UpdateProjectRate(ctx, BusinessID(1), 1, 1, "1020"); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}
