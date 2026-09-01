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
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
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

func (o *ExpenseCategoryListOptions) requestOptions() []RequestOption {
	if o == nil {
		return nil
	}
	var opts []RequestOption
	if o.Search != nil {
		opts = append(opts, o.Search)
	}
	if o.Page > 0 {
		opts = append(opts, PageNumber(o.Page))
	}
	if o.PerPage > 0 {
		opts = append(opts, PerPage(o.PerPage))
	}
	return opts
}

func expenseCategoriesPath(acct AccountID) string {
	return fmt.Sprintf("/accounting/account/%s/expenses/categories", acct)
}

func expenseCategoryPath(acct AccountID, id int64) string {
	return fmt.Sprintf("/accounting/account/%s/expenses/categories/%d", acct, id)
}

// List returns one page of expense categories.
//
// inventory: Expenses/List Expense Categories
func (s *ExpenseCategoriesService) List(ctx context.Context, acct AccountID, opts *ExpenseCategoryListOptions) (*Page[ExpenseCategory], error) {
	var env expenseCategoryListEnvelope
	if err := s.client.do(ctx, http.MethodGet, expenseCategoriesPath(acct), FamilyAccounting, nil, &env, opts.requestOptions()...); err != nil {
		return nil, err
	}
	return &Page[ExpenseCategory]{Items: env.Categories, Page: env.Page, Pages: env.Pages, PerPage: env.PerPage, Total: env.Total}, nil
}

// All walks every page of expense categories, auto-paginating.
func (s *ExpenseCategoriesService) All(ctx context.Context, acct AccountID, opts *ExpenseCategoryListOptions) iter.Seq2[ExpenseCategory, error] {
	perPage := 100
	var search Search
	if opts != nil {
		if opts.PerPage > 0 {
			perPage = opts.PerPage
		}
		search = opts.Search
	}
	return All(ctx, func(ctx context.Context, page int) (*Page[ExpenseCategory], error) {
		return s.List(ctx, acct, &ExpenseCategoryListOptions{Search: search, Page: page, PerPage: perPage})
	})
}

// Get retrieves a single expense category.
//
// inventory: Expenses/Single Expense Category
func (s *ExpenseCategoriesService) Get(ctx context.Context, acct AccountID, id int64) (*ExpenseCategory, error) {
	var env expenseCategoryEnvelope
	if err := s.client.do(ctx, http.MethodGet, expenseCategoryPath(acct, id), FamilyAccounting, nil, &env); err != nil {
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
	body := struct {
		Category *ExpenseCategoryCreateRequest `json:"category"`
	}{req}
	var env expenseCategoryEnvelope
	if err := s.client.do(ctx, http.MethodPost, expenseCategoriesPath(acct), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Category, nil
}
