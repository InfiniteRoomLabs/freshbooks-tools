package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// tasksCommands wrap *freshbooks.TasksService.
var tasksCommands = []Command{
	{
		Group: "tasks", Verb: "create",
		Short:   "Create a project task",
		Service: "Tasks", Method: "Create",
		Keys:  []string{"Projects/Tasks/Create Task"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.TaskCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Tasks.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "tasks", Verb: "get",
		Short:   "Get a single task",
		Service: "Tasks", Method: "Get",
		Keys:  []string{"Projects/Tasks/Single Task"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Tasks.Get(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "tasks", Verb: "list",
		Short:   "List tasks",
		Service: "Tasks", Method: "List",
		Keys:  []string{"Projects/Tasks/List Tasks"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.TaskListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Tasks.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.Tasks.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "tasks", Verb: "update",
		Short:   "Update a task",
		Service: "Tasks", Method: "Update",
		Keys:  []string{"Projects/Tasks/Update Task"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.TaskUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Tasks.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "tasks", Verb: "delete",
		Short:   "Delete a task",
		Service: "Tasks", Method: "Delete",
		Keys:  []string{"Projects/Tasks/Delete Task"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Tasks.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
