package freshbooks

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// pagedFetch returns a fetch function serving pages of ints.
func pagedFetch(pages [][]int, total int) func(context.Context, int) (*Page[int], error) {
	return func(_ context.Context, n int) (*Page[int], error) {
		if n > len(pages) {
			return &Page[int]{Page: n, Pages: len(pages), Total: total}, nil
		}
		return &Page[int]{Items: pages[n-1], Page: n, Pages: len(pages), PerPage: 2, Total: total}, nil
	}
}

func TestAll(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] walks every page in order", func(t *testing.T) {
		var got []int
		for v, err := range All(ctx, pagedFetch([][]int{{1, 2}, {3, 4}, {5}}, 5)) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, v)
		}
		if fmt.Sprint(got) != "[1 2 3 4 5]" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("[edge] an empty first page ends the walk", func(t *testing.T) {
		n := 0
		for range All(ctx, pagedFetch([][]int{{}}, 0)) {
			n++
		}
		if n != 0 {
			t.Fatalf("yielded %d items", n)
		}
	})

	t.Run("[edge] a server that never reports Pages stops on an empty page", func(t *testing.T) {
		fetch := func(_ context.Context, n int) (*Page[int], error) {
			if n > 2 {
				return &Page[int]{}, nil
			}
			return &Page[int]{Items: []int{n}}, nil
		}
		var got []int
		for v, err := range All(ctx, fetch) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, v)
		}
		if fmt.Sprint(got) != "[1 2]" {
			t.Fatalf("got %v", got)
		}
	})

	t.Run("[edge] a nil page ends the walk", func(t *testing.T) {
		n := 0
		for range All(ctx, func(context.Context, int) (*Page[int], error) { return nil, nil }) {
			n++
		}
		if n != 0 {
			t.Fatalf("yielded %d items", n)
		}
	})

	t.Run("[sad] a mid-stream error is yielded once and ends the walk", func(t *testing.T) {
		boom := errors.New("boom")
		fetch := func(_ context.Context, n int) (*Page[int], error) {
			if n == 2 {
				return nil, boom
			}
			return &Page[int]{Items: []int{1, 2}, Page: n, Pages: 3}, nil
		}
		var values []int
		var errs []error
		for v, err := range All(ctx, fetch) {
			if err != nil {
				errs = append(errs, err)
				continue
			}
			values = append(values, v)
		}
		if fmt.Sprint(values) != "[1 2]" {
			t.Fatalf("values = %v", values)
		}
		if len(errs) != 1 || !errors.Is(errs[0], boom) {
			t.Fatalf("errs = %v", errs)
		}
	})

	t.Run("[sad] a cancelled context is reported before the first fetch", func(t *testing.T) {
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		fetched := false
		var gotErr error
		for _, err := range All(cancelled, func(context.Context, int) (*Page[int], error) {
			fetched = true
			return &Page[int]{}, nil
		}) {
			gotErr = err
		}
		if fetched {
			t.Fatal("fetch was called with a cancelled context")
		}
		if !errors.Is(gotErr, context.Canceled) {
			t.Fatalf("err = %v", gotErr)
		}
	})

	t.Run("[happy] breaking out of the range stops fetching", func(t *testing.T) {
		calls := 0
		fetch := func(_ context.Context, n int) (*Page[int], error) {
			calls++
			return &Page[int]{Items: []int{1, 2}, Page: n, Pages: 100}, nil
		}
		for v := range All(ctx, fetch) {
			_ = v
			break
		}
		if calls != 1 {
			t.Fatalf("fetched %d pages after an early break", calls)
		}
	})
}

func TestPageMeta(t *testing.T) {
	// PageMeta exists so resource list structs can embed the pagination
	// block both families return; assert the tags it will be decoded with.
	m := PageMeta{Page: 1, Pages: 2, PerPage: 15, Total: 20}
	if m.Page != 1 || m.Pages != 2 || m.PerPage != 15 || m.Total != 20 {
		t.Fatalf("PageMeta = %+v", m)
	}
}
