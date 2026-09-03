package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// timeEntriesCommands wrap *freshbooks.TimeEntriesService.
var timeEntriesCommands = []Command{
	{
		Group: "time-entries", Verb: "list",
		Short:   "List time entries",
		Service: "TimeEntries", Method: "List",
		Keys:  []string{"Time Tracking/List Entries", "Time Tracking/Time Entries Updated Since Precise Time", "Time Tracking/Time Entries for a Given Day"},
		Class: ClassRO, Scope: ScopeBusiness, List: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.TimeEntryListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			return c.TimeEntries.List(ctx, inv.Scope.BusinessID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "time-entries", Verb: "search",
		Short:   "Search time entries for an employee on a specific project",
		Service: "TimeEntries", Method: "Search",
		Keys:  []string{"Time Tracking/Time Entries For Employee on Specific Project"},
		Class: ClassRO, Scope: ScopeBusiness, List: true, ExtraPositional: []string{"query"},
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.TimeEntryListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			return c.TimeEntries.Search(ctx, inv.Scope.BusinessID, inv.Extra(0), opts)
		},
	},
	{
		Group: "time-entries", Verb: "create",
		Short:   "Create a time entry",
		Service: "TimeEntries", Method: "Create",
		Keys:  []string{"Time Tracking/Create a Time Entry"},
		Class: ClassW, Scope: ScopeBusiness, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.TimeEntryCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.TimeEntries.Create(ctx, inv.Scope.BusinessID, &body)
		},
	},
	{
		Group: "time-entries", Verb: "update",
		Short:   "Update a time entry",
		Service: "TimeEntries", Method: "Update",
		Keys:  []string{"Time Tracking/Update a Time Entry"},
		Class: ClassI, Scope: ScopeBusiness, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.TimeEntryUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.TimeEntries.Update(ctx, inv.Scope.BusinessID, inv.IntID(), &body)
		},
	},
	{
		Group: "time-entries", Verb: "delete",
		Short:   "Delete a time entry",
		Service: "TimeEntries", Method: "Delete",
		Keys:  []string{"Time Tracking/Delete a Time Entry"},
		Class: ClassD, Scope: ScopeBusiness, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.TimeEntries.Delete(ctx, inv.Scope.BusinessID, inv.IntID()))
		},
	},
	{
		// Wraps ListWithTotals (Phase 8 convergence), the same wire
		// endpoint as "list" above, decoded for its business-wide
		// logged/unbilled totals too -- not a distinct Postman request, so
		// it carries no inventory key of its own (mirrors
		// time_entries_list_with_totals in mcp/internal/tools).
		Group: "time-entries", Verb: "list-with-totals",
		Short:   "List time entries with logged/unbilled totals (totals in -o json or -o yaml only; table mode shows the entries)",
		Service: "TimeEntries", Method: "ListWithTotals",
		Class: ClassRO, Scope: ScopeBusiness, List: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.TimeEntries.ListWithTotals(ctx, inv.Scope.BusinessID, inv.ReqOpts()...)
		},
	},
}
