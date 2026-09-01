package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// ExpenseCategoriesService is the expense-categories resource: the taxonomy
// expenses are filed under.
//
// The Postman collection includes a "Create Custom Expense Category"
// request, but the FreshBooks docs page for this resource states plainly
// that creating, updating, and deleting categories is not supported by the
// API. Create is implemented here to honor the inventory (and because a
// custom-category feature exists in the FreshBooks UI, so a write endpoint
// plausibly does too), but the fact is INFERRED from Postman only and
// contradicts the docs; see the spec's STATE AS OF callout in section 3.
type ExpenseCategoriesService struct{ client *Client }

// ExpenseCategory is an expense category, either a FreshBooks system
// default or a custom one.
type ExpenseCategory struct {
	// ID and CategoryID are the same value under two names; the API returns
	// both.
	ID         int64 `json:"id"`
	CategoryID int64 `json:"categoryid"`
	// Category is the display name, e.g. "Advertising".
	Category string `json:"category"`
	// ParentID is the parent category, or zero for a top-level category.
	ParentID int64 `json:"parentid,omitempty"`
	// IsEditable reports whether this category's name can be changed.
	IsEditable bool `json:"is_editable,omitempty"`
	// IsCOGS reports whether this category counts toward cost of goods
	// sold. FreshBooks docs mark this field deprecated.
	IsCOGS bool `json:"is_cogs,omitempty"`
	// CreatedAt and UpdatedAt are account-local timestamps.
	CreatedAt DateTime `json:"created_at,omitempty"`
	UpdatedAt DateTime `json:"updated_at,omitempty"`
	// VisState is the category's visibility state.
	VisState VisState `json:"vis_state"`
}

type expenseCategoryEnvelope struct {
	Category ExpenseCategory `json:"category"`
}

type expenseCategoryListEnvelope struct {
	Categories []ExpenseCategory `json:"categories"`
	PageMeta
}

// ExpenseCategoryCreateRequest is the payload for Create.
type ExpenseCategoryCreateRequest struct {
	Category string `json:"category"`
	ParentID *int64 `json:"parentid,omitempty"`
	IsCOGS   *bool  `json:"is_cogs,omitempty"`
}

// ExpenseCategoryListOptions filters and paginates List.
type ExpenseCategoryListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *ExpenseCategoryListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

func expenseCategoriesPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/expenses/categories", acct), nil
}

func expenseCategoryPath(acct AccountID, id int64) (string, error) {
	base, err := expenseCategoriesPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
}

// List returns one page of expense categories.
//
// inventory: Expenses/List Expense Categories
func (s *ExpenseCategoriesService) List(ctx context.Context, acct AccountID, opts *ExpenseCategoryListOptions, extra ...RequestOption) (*Page[ExpenseCategory], error) {
	path, err := expenseCategoriesPath(acct)
	if err != nil {
		return nil, err
	}
	var env expenseCategoryListEnvelope
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(env.Categories, env.PageMeta), nil
}

// All walks every page of expense categories, auto-paginating.
func (s *ExpenseCategoriesService) All(ctx context.Context, acct AccountID, opts *ExpenseCategoryListOptions, extra ...RequestOption) iter.Seq2[ExpenseCategory, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[ExpenseCategory], error) {
		o := ExpenseCategoryListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		o.PerPage = pageSize(o.PerPage)
		return s.List(ctx, acct, &o, extra...)
	})
}

// Get retrieves a single expense category.
//
// inventory: Expenses/Single Expense Category
func (s *ExpenseCategoriesService) Get(ctx context.Context, acct AccountID, id int64, opts ...RequestOption) (*ExpenseCategory, error) {
	path, err := expenseCategoryPath(acct, id)
	if err != nil {
		return nil, err
	}
	var env expenseCategoryEnvelope
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &env, opts...); err != nil {
		return nil, err
	}
	return &env.Category, nil
}

// Create adds a custom expense category. See the type doc comment: this
// contradicts the FreshBooks docs page, which says category creation is
// unsupported.
//
// inventory: Expenses/Create Custom Expense Category
func (s *ExpenseCategoriesService) Create(ctx context.Context, acct AccountID, req *ExpenseCategoryCreateRequest) (*ExpenseCategory, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: ExpenseCategories.Create needs a request")
	}
	path, err := expenseCategoriesPath(acct)
	if err != nil {
		return nil, err
	}
	body := struct {
		Category *ExpenseCategoryCreateRequest `json:"category"`
	}{req}
	var env expenseCategoryEnvelope
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Category, nil
}
