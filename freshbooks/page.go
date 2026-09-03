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
	Items []T `json:"items"`
	// Page is the 1-based index of this page.
	Page int `json:"page"`
	// Pages is the total number of pages.
	Pages int `json:"pages"`
	// PerPage is the page size the server used.
	PerPage int `json:"per_page"`
	// Total is the total number of matching resources.
	Total int `json:"total"`
	// Sort is the sort the server reports it applied, echoed back from the
	// request's "sort" parameter. Only the business-scoped family sends it
	// (in meta.sort), so it is nil for every accounting-family list.
	//
	// It is an echo, not a validation: the API repeats whatever it was
	// given, including a field that does not exist, and answers 200 either
	// way (CONFIRMED live, 2026-09-03). Read it to see what was asked for,
	// never as proof the sort was understood.
	Sort []string `json:"sort,omitempty"`
}

// PageMeta is the pagination block both families return, ready to embed in a
// resource's list-response struct.
type PageMeta struct {
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
	// Sort is the business-scoped family's meta.sort echo; see Page.Sort.
	// The accounting family does not send it, so it stays nil there.
	Sort []string `json:"sort,omitempty"`
}

// listOpts is the option plumbing every resource's list-options struct
// shares: a filter, a page number, and a page size, each omitted when
// unset. Every *ListOptions.opts method in the package delegates here so
// the five (and counting) sibling structs stay in lockstep.
func listOpts(search Search, page, perPage int) []RequestOption {
	var opts []RequestOption
	if len(search) > 0 {
		opts = append(opts, search)
	}
	if page > 0 {
		opts = append(opts, PageNumber(page))
	}
	if perPage > 0 {
		opts = append(opts, PerPage(perPage))
	}
	return opts
}

// defaultPerPage is the page size All uses when the caller did not pick one:
// large enough to keep the round-trip count down, small enough to stay
// inside the accounting family's page-size ceiling.
const defaultPerPage = 100

// pageSize resolves a caller's page size against defaultPerPage.
func pageSize(perPage int) int {
	if perPage > 0 {
		return perPage
	}
	return defaultPerPage
}

// newPage assembles a Page from a decoded list response's items and its
// pagination block, whichever family the block came from (an embedded
// PageMeta for the accounting family, a "meta" object for the business
// family).
func newPage[T any](items []T, m PageMeta) *Page[T] {
	return &Page[T]{Items: items, Page: m.Page, Pages: m.Pages, PerPage: m.PerPage, Total: m.Total, Sort: m.Sort}
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
