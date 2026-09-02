package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type projectsCreateIn struct {
	BizScope
	Body freshbooks.ProjectCreateRequest `json:"body" jsonschema:"the project fields to create"`
}

type projectsIDIn struct {
	BizScope
	ID int64 `json:"id" jsonschema:"the project id"`
}

type projectsListIn struct {
	BizScope
	listIn
}

type projectsUpdateIn struct {
	BizScope
	ID   int64                           `json:"id" jsonschema:"the project id"`
	Body freshbooks.ProjectUpdateRequest `json:"body" jsonschema:"the project fields to update"`
}

type projectsCreateThreadIn struct {
	BizScope
	ProjectID int64  `json:"project_id" jsonschema:"the project id"`
	Message   string `json:"message" jsonschema:"the discussion message"`
}

type projectsAddThreadCommentIn struct {
	BizScope
	ThreadID int64  `json:"thread_id" jsonschema:"the discussion thread id"`
	Content  string `json:"content" jsonschema:"the comment content"`
}

// projectsSpecs are the tools wrapping *freshbooks.ProjectsService.
var projectsSpecs = []Spec{
	newSpec("projects_create",
		"Create a project. See https://www.freshbooks.com/api/projects.",
		"Projects", "Create",
		[]string{"Projects/Create Single Project"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in projectsCreateIn) (any, error) {
			return c.Projects.Create(ctx, scope.BusinessID, &in.Body)
		}),
	newSpec("projects_get",
		"Get a single project. See https://www.freshbooks.com/api/projects.",
		"Projects", "Get",
		[]string{"Projects/Single Project"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in projectsIDIn) (any, error) {
			return c.Projects.Get(ctx, scope.BusinessID, in.ID)
		}),
	newSpec("projects_list",
		"List a business's projects. See https://www.freshbooks.com/api/projects.",
		"Projects", "List",
		[]string{"Projects/List Projects"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in projectsListIn) (any, error) {
			return c.Projects.List(ctx, scope.BusinessID, &freshbooks.ProjectListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("projects_update",
		"Update a project. See https://www.freshbooks.com/api/projects.",
		"Projects", "Update",
		[]string{"Projects/Update Project"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in projectsUpdateIn) (any, error) {
			return c.Projects.Update(ctx, scope.BusinessID, in.ID, &in.Body)
		}),
	newSpec("projects_delete",
		"Delete a project. See https://www.freshbooks.com/api/projects.",
		"Projects", "Delete",
		[]string{"Projects/Delete Project"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in projectsIDIn) (any, error) {
			return void(c.Projects.Delete(ctx, scope.BusinessID, in.ID))
		}),
	newSpec("projects_abilities",
		"List a business's project permission abilities. See https://www.freshbooks.com/api/projects.",
		"Projects", "Abilities",
		[]string{"Settings/Abilities/Abilities"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in BizScope) (any, error) {
			return c.Projects.Abilities(ctx, scope.BusinessID)
		}),
	newSpec("projects_threads",
		"List every message in a project's discussion. See https://www.freshbooks.com/api/projects.",
		"Projects", "Threads",
		[]string{"Projects/List All Messages in Project Discussion"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in projectsIDIn) (any, error) {
			return c.Projects.Threads(ctx, scope.BusinessID, in.ID)
		}),
	newSpec("projects_create_thread",
		"Start a new message in a project's discussion. See https://www.freshbooks.com/api/projects.",
		"Projects", "CreateThread",
		[]string{"Projects/Create New Message in Project Discussion"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in projectsCreateThreadIn) (any, error) {
			return c.Projects.CreateThread(ctx, scope.BusinessID, in.ProjectID, in.Message)
		}),
	newSpec("projects_add_thread_comment",
		"Add a comment to a project discussion message. See https://www.freshbooks.com/api/projects.",
		"Projects", "AddThreadComment",
		[]string{"Projects/Add Comment to Project Discussion Message"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in projectsAddThreadCommentIn) (any, error) {
			return c.Projects.AddThreadComment(ctx, scope.BusinessID, in.ThreadID, in.Content)
		}),
}
