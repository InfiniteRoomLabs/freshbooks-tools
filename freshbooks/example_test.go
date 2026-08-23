package freshbooks_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// Example shows the shape of every program built on this library: a token
// source, a client, then a call that resolves the identifiers the rest of
// the API needs.
//
// A real program would point the client at the default base URL and build
// the token source with auth.NewTokenSource over a persistent store; the
// fixture server here only keeps the example runnable and deterministic.
func Example() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"response": {
			"id": 4242424,
			"email": "owner@example.com",
			"business_memberships": [
				{"role": "owner", "business": {
					"id": 8675309,
					"account_id": "ACM123",
					"business_uuid": "00000000-0000-4000-8000-000000000001",
					"name": "Example Business LLC"
				}}
			]
		}}`)
	}))
	defer srv.Close()

	client, err := freshbooks.NewClient(
		freshbooks.WithTokenSource(auth.StaticTokenSource("an-access-token")),
		freshbooks.WithUserAgent("example-app/1.0"),
		freshbooks.WithBaseURL(srv.URL),
	)
	if err != nil {
		log.Fatal(err)
	}

	memberships, err := client.Identity.Me(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	for _, m := range memberships {
		// AccountID addresses the accounting family, BusinessID the
		// business-scoped family. They are never interchangeable.
		fmt.Printf("%s: account %s, business %d\n", m.Name, m.AccountID, m.BusinessID)
	}
	// Output:
	// Example Business LLC: account ACM123, business 8675309
}

// ExampleAll shows the auto-paginating iterator a resource service's All
// method wraps: it walks pages until the server runs out, and reports the
// first error once.
func ExampleAll() {
	pages := [][]string{{"first", "second"}, {"third"}}
	fetch := func(_ context.Context, page int) (*freshbooks.Page[string], error) {
		if page > len(pages) {
			return &freshbooks.Page[string]{Page: page, Pages: len(pages)}, nil
		}
		return &freshbooks.Page[string]{
			Items: pages[page-1],
			Page:  page,
			Pages: len(pages),
			Total: 3,
		}, nil
	}

	for name, err := range freshbooks.All(context.Background(), fetch) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(name)
	}
	// Output:
	// first
	// second
	// third
}
