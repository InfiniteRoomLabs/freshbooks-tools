package tools

import (
	"context"
	"encoding/json"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// retainersListIn deliberately does not embed listIn: RetainerListOptions
// carries only Search, no Page/PerPage (see freshbooks/retainers.go and
// docs/phases/3/plan.md's gotchas -- inventing pagination fields the lib
// does not support is exactly what that note warns against).
type retainersListIn struct {
	BizScope
	Search map[string]string `json:"search,omitempty" jsonschema:"filter fields as key/value pairs"`
}

type retainersIDIn struct {
	BizScope
	idIn
}

// retainersCreateIn and retainersUpdateIn do not embed
// freshbooks.Retainer{Create,Update}Request whole: both carry Fee and
// ExcessRate as json.Number, which jsonschema-go infers as JSON type
// "string" (json.Number's reflect Kind()) -- but go-sdk's decoder
// (github.com/segmentio/encoding/json, see internal/json/json.go) accepts
// only an unquoted numeric literal for a json.Number field and rejects a
// quoted string outright. No JSON value satisfies both the inferred schema
// and the decoder at once, so Fee and ExcessRate are exposed as plain
// strings here and converted with json.Number(...) in the Call closure.
// Not one of the "reports, uploads, thread comments, rates, verify,
// checkout links, tokenization" categories docs/phases/3/plan.md's D3
// anticipated; discovered and resolved during round-trip testing (see
// docs/phases/3/reports/implementer.md).
type retainersCreateIn struct {
	BizScope
	ClientUserID          string `json:"client_user_id" jsonschema:"the client's FreshBooks user id"`
	StartDate             string `json:"start_date" jsonschema:"the retainer's start date, YYYY-MM-DD"`
	NextPeriodStartDate   string `json:"next_period_start_date,omitempty" jsonschema:"the next period's start date, YYYY-MM-DD"`
	Fee                   string `json:"fee" jsonschema:"the retainer fee, as a decimal string"`
	ExcessRate            string `json:"excess_rate,omitempty" jsonschema:"the excess-usage rate, as a decimal string"`
	BudgetedTime          int64  `json:"budgeted_time,omitempty" jsonschema:"budgeted minutes"`
	Active                bool   `json:"active" jsonschema:"whether the retainer is active"`
	Frequency             string `json:"frequency,omitempty" jsonschema:"the billing frequency"`
	NumberRecurring       int    `json:"number_recurring,omitempty" jsonschema:"how many times the retainer recurs"`
	IsInfinitelyRecurring bool   `json:"is_infinitely_recurring,omitempty" jsonschema:"whether the retainer recurs indefinitely"`
}

func (in retainersCreateIn) body() *freshbooks.RetainerCreateRequest {
	return &freshbooks.RetainerCreateRequest{
		ClientUserID:          in.ClientUserID,
		StartDate:             in.StartDate,
		NextPeriodStartDate:   in.NextPeriodStartDate,
		Fee:                   json.Number(in.Fee),
		ExcessRate:            json.Number(in.ExcessRate),
		BudgetedTime:          in.BudgetedTime,
		Active:                in.Active,
		Frequency:             in.Frequency,
		NumberRecurring:       in.NumberRecurring,
		IsInfinitelyRecurring: in.IsInfinitelyRecurring,
	}
}

type retainersUpdateIn struct {
	BizScope
	idIn
	ClientUserID          string `json:"client_user_id,omitempty" jsonschema:"the client's FreshBooks user id"`
	StartDate             string `json:"start_date,omitempty" jsonschema:"the retainer's start date, YYYY-MM-DD"`
	NextPeriodStartDate   string `json:"next_period_start_date,omitempty" jsonschema:"the next period's start date, YYYY-MM-DD"`
	Fee                   string `json:"fee,omitempty" jsonschema:"the retainer fee, as a decimal string"`
	ExcessRate            string `json:"excess_rate,omitempty" jsonschema:"the excess-usage rate, as a decimal string"`
	BudgetedTime          int64  `json:"budgeted_time,omitempty" jsonschema:"budgeted minutes"`
	Active                *bool  `json:"active,omitempty" jsonschema:"whether the retainer is active"`
	Frequency             string `json:"frequency,omitempty" jsonschema:"the billing frequency"`
	NumberRecurring       int    `json:"number_recurring,omitempty" jsonschema:"how many times the retainer recurs"`
	IsInfinitelyRecurring bool   `json:"is_infinitely_recurring,omitempty" jsonschema:"whether the retainer recurs indefinitely"`
}

func (in retainersUpdateIn) body() *freshbooks.RetainerUpdateRequest {
	return &freshbooks.RetainerUpdateRequest{
		ClientUserID:          in.ClientUserID,
		StartDate:             in.StartDate,
		NextPeriodStartDate:   in.NextPeriodStartDate,
		Fee:                   json.Number(in.Fee),
		ExcessRate:            json.Number(in.ExcessRate),
		BudgetedTime:          in.BudgetedTime,
		Active:                in.Active,
		Frequency:             in.Frequency,
		NumberRecurring:       in.NumberRecurring,
		IsInfinitelyRecurring: in.IsInfinitelyRecurring,
	}
}

// retainersSpecs are the tools wrapping *freshbooks.RetainersService.
var retainersSpecs = []Spec{
	newSpec("retainers_list",
		"List a business's retainers. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "List",
		[]string{"Invoices/Retainers/Get all retainers"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersListIn) (any, error) {
			var search freshbooks.Search
			if len(in.Search) > 0 {
				search = freshbooks.Search(in.Search)
			}
			return c.Retainers.List(ctx, scope.BusinessID, &freshbooks.RetainerListOptions{Search: search})
		}),
	newSpec("retainers_get",
		"Get a single retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Get",
		[]string{"Invoices/Retainers/Single Retainer"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersIDIn) (any, error) {
			return c.Retainers.Get(ctx, scope.BusinessID, in.ID)
		}),
	newSpec("retainers_create",
		"Create a retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Create",
		[]string{"Invoices/Retainers/Create Retainer"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersCreateIn) (any, error) {
			return c.Retainers.Create(ctx, scope.BusinessID, in.body())
		}),
	newSpec("retainers_update",
		"Update a retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Update",
		[]string{"Invoices/Retainers/Update Retainer"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersUpdateIn) (any, error) {
			return c.Retainers.Update(ctx, scope.BusinessID, in.ID, in.body())
		}),
	newSpec("retainers_delete",
		"Delete a retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Delete",
		[]string{"Invoices/Retainers/Delete Retainer"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersIDIn) (any, error) {
			return c.Retainers.Delete(ctx, scope.BusinessID, in.ID)
		}),
	newSpec("retainers_undelete",
		"Restore a deleted retainer. See https://www.freshbooks.com/api/invoices.",
		"Retainers", "Undelete",
		[]string{"Invoices/Retainers/Undelete Retainer"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in retainersIDIn) (any, error) {
			return c.Retainers.Undelete(ctx, scope.BusinessID, in.ID)
		}),
}
