package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type timeEntriesListIn struct {
	BizScope
	listIn
}

type timeEntriesSearchIn struct {
	BizScope
	Query string `json:"query" jsonschema:"the employee/project search query"`
	listIn
}

type timeEntriesCreateIn struct {
	BizScope
	Body freshbooks.TimeEntryCreateRequest `json:"body" jsonschema:"the time entry fields to create"`
}

type timeEntriesIDIn struct {
	BizScope
	ID int64 `json:"id" jsonschema:"the time entry id"`
}

type timeEntriesUpdateIn struct {
	BizScope
	ID   int64                             `json:"id" jsonschema:"the time entry id"`
	Body freshbooks.TimeEntryUpdateRequest `json:"body" jsonschema:"the time entry fields to update"`
}

// timeEntriesSpecs are the tools wrapping *freshbooks.TimeEntriesService.
var timeEntriesSpecs = []Spec{
	newSpec("time_entries_list",
		"List a business's time entries. See https://www.freshbooks.com/api/time_tracking.",
		"TimeEntries", "List",
		[]string{"Time Tracking/List Entries", "Time Tracking/Time Entries Updated Since Precise Time", "Time Tracking/Time Entries for a Given Day"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in timeEntriesListIn) (any, error) {
			return c.TimeEntries.List(ctx, scope.BusinessID, &freshbooks.TimeEntryListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("time_entries_search",
		"Search a business's time entries for an employee on a specific project. See https://www.freshbooks.com/api/time_tracking.",
		"TimeEntries", "Search",
		[]string{"Time Tracking/Time Entries For Employee on Specific Project"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in timeEntriesSearchIn) (any, error) {
			return c.TimeEntries.Search(ctx, scope.BusinessID, in.Query, &freshbooks.TimeEntryListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("time_entries_create",
		"Create a time entry. See https://www.freshbooks.com/api/time_tracking.",
		"TimeEntries", "Create",
		[]string{"Time Tracking/Create a Time Entry"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in timeEntriesCreateIn) (any, error) {
			return c.TimeEntries.Create(ctx, scope.BusinessID, &in.Body)
		}),
	newSpec("time_entries_update",
		"Update a time entry. See https://www.freshbooks.com/api/time_tracking.",
		"TimeEntries", "Update",
		[]string{"Time Tracking/Update a Time Entry"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in timeEntriesUpdateIn) (any, error) {
			return c.TimeEntries.Update(ctx, scope.BusinessID, in.ID, &in.Body)
		}),
	newSpec("time_entries_delete",
		"Delete a time entry. See https://www.freshbooks.com/api/time_tracking.",
		"TimeEntries", "Delete",
		[]string{"Time Tracking/Delete a Time Entry"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in timeEntriesIDIn) (any, error) {
			return void(c.TimeEntries.Delete(ctx, scope.BusinessID, in.ID))
		}),
	// time_entries_list_with_totals wraps ListWithTotals (Phase 8
	// convergence), not a distinct Postman request -- same endpoint as
	// time_entries_list, decoded for its totals too. Keyless, like
	// identity_whoami: it carries no inventory key of its own.
	newSpec("time_entries_list_with_totals",
		"List a business's time entries along with the business-wide logged/unbilled totals FreshBooks reports alongside them. See https://www.freshbooks.com/api/time_tracking.",
		"TimeEntries", "ListWithTotals",
		nil, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in timeEntriesListIn) (any, error) {
			return c.TimeEntries.ListWithTotals(ctx, scope.BusinessID, in.reqOpts()...)
		}),
}
