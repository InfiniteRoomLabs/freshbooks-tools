package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type tasksCreateIn struct {
	AcctScope
	Body freshbooks.TaskCreateRequest `json:"body" jsonschema:"the task fields to create"`
}

type tasksIDIn struct {
	AcctScope
	idIn
}

type tasksListIn struct {
	AcctScope
	listIn
}

type tasksUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.TaskUpdateRequest `json:"body" jsonschema:"the task fields to update"`
}

// tasksSpecs are the tools wrapping *freshbooks.TasksService.
var tasksSpecs = []Spec{
	newSpec("tasks_create",
		"Create a project task. See https://www.freshbooks.com/api/projects.",
		"Tasks", "Create",
		[]string{"Projects/Tasks/Create Task"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in tasksCreateIn) (any, error) {
			return c.Tasks.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("tasks_get",
		"Get a single task. See https://www.freshbooks.com/api/projects.",
		"Tasks", "Get",
		[]string{"Projects/Tasks/Single Task"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in tasksIDIn) (any, error) {
			return c.Tasks.Get(ctx, scope.AccountID, in.ID)
		}),
	newSpec("tasks_list",
		"List an account's tasks. See https://www.freshbooks.com/api/projects.",
		"Tasks", "List",
		[]string{"Projects/Tasks/List Tasks"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in tasksListIn) (any, error) {
			return c.Tasks.List(ctx, scope.AccountID, &freshbooks.TaskListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("tasks_update",
		"Update a task. See https://www.freshbooks.com/api/projects.",
		"Tasks", "Update",
		[]string{"Projects/Tasks/Update Task"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in tasksUpdateIn) (any, error) {
			return c.Tasks.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("tasks_delete",
		"Delete a task. See https://www.freshbooks.com/api/projects.",
		"Tasks", "Delete",
		[]string{"Projects/Tasks/Delete Task"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in tasksIDIn) (any, error) {
			return void(c.Tasks.Delete(ctx, scope.AccountID, in.ID))
		}),
}
