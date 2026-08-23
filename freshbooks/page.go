package freshbooks

import (
	"context"
	"iter"
)

// Page is one page of a list response, normalized across both API families:
// the accounting family reports page/pages/per_page/total inside its result
// envelope, the business-scoped family reports the same fields in a "meta"
// object.
type Page[T any] struct {
	// Items are the resources on this page.
	Items []T
	// Page is the 1-based index of this page.
	Page int
	// Pages is the total number of pages.
	Pages int
	// PerPage is the page size the server used.
	PerPage int
	// Total is the total number of matching resources.
	Total int
}

// PageMeta is the pagination block both families return, ready to embed in a
// resource's list-response struct.
type PageMeta struct {
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

// All walks every page of a list endpoint and yields each item in order. It
// stops at the first error -- yielding it once, with the zero item -- and at
// ctx cancellation. Resource services wrap it to provide their own All
// method; fetch is called with 1-based page numbers.
func All[T any](ctx context.Context, fetch func(ctx context.Context, page int) (*Page[T], error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		for page := 1; ; page++ {
			if err := ctx.Err(); err != nil {
				yield(zero, err)
				return
			}
			p, err := fetch(ctx, page)
			if err != nil {
				yield(zero, err)
				return
			}
			if p == nil {
				return
			}
			for _, item := range p.Items {
				if !yield(item, nil) {
					return
				}
			}
			if len(p.Items) == 0 || (p.Pages > 0 && page >= p.Pages) {
				return
			}
		}
	}
}
