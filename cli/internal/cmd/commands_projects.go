package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/pflag"
)

// projectsCommands wrap *freshbooks.ProjectsService.
var projectsCommands = []Command{
	{
		Group: "projects", Verb: "create",
		Short:   "Create a project",
		Service: "Projects", Method: "Create",
		Keys:  []string{"Projects/Create Single Project"},
		Class: ClassW, Scope: ScopeBusiness, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ProjectCreateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Projects.Create(ctx, inv.Scope.BusinessID, &body)
		},
	},
	{
		Group: "projects", Verb: "get",
		Short:   "Get a single project",
		Service: "Projects", Method: "Get",
		Keys:  []string{"Projects/Single Project"},
		Class: ClassRO, Scope: ScopeBusiness, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Projects.Get(ctx, inv.Scope.BusinessID, inv.IntID())
		},
	},
	{
		Group: "projects", Verb: "list",
		Short:   "List projects",
		Service: "Projects", Method: "List",
		Keys:  []string{"Projects/List Projects"},
		Class: ClassRO, Scope: ScopeBusiness, List: true, HasAll: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.ProjectListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.Projects.All(ctx, inv.Scope.BusinessID, opts))
			}
			return c.Projects.List(ctx, inv.Scope.BusinessID, opts)
		},
	},
	{
		Group: "projects", Verb: "update",
		Short:   "Update a project",
		Service: "Projects", Method: "Update",
		Keys:  []string{"Projects/Update Project"},
		Class: ClassI, Scope: ScopeBusiness, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.ProjectUpdateRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Projects.Update(ctx, inv.Scope.BusinessID, inv.IntID(), &body)
		},
	},
	{
		Group: "projects", Verb: "delete",
		Short:   "Delete a project",
		Service: "Projects", Method: "Delete",
		Keys:  []string{"Projects/Delete Project"},
		Class: ClassD, Scope: ScopeBusiness, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Projects.Delete(ctx, inv.Scope.BusinessID, inv.IntID()))
		},
	},
	{
		Group: "projects", Verb: "abilities",
		Short:   "List a business's project abilities",
		Service: "Projects", Method: "Abilities",
		Keys:  []string{"Settings/Abilities/Abilities"},
		Class: ClassRO, Scope: ScopeBusiness,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Projects.Abilities(ctx, inv.Scope.BusinessID)
		},
	},
	{
		Group: "projects", Verb: "threads",
		Short:   "List a project's discussion messages",
		Service: "Projects", Method: "Threads",
		Keys:  []string{"Projects/List All Messages in Project Discussion"},
		Class: ClassRO, Scope: ScopeBusiness, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Projects.Threads(ctx, inv.Scope.BusinessID, inv.IntID())
		},
	},
	{
		Group: "projects", Verb: "create-thread",
		Short:   "Post a new message in a project's discussion",
		Service: "Projects", Method: "CreateThread",
		Keys:  []string{"Projects/Create New Message in Project Discussion"},
		Class: ClassW, Scope: ScopeBusiness, HasID: true,
		ExtraFlags: func(fs *pflag.FlagSet) {
			fs.String("message", "", "the message text (required)")
		},
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			msg, _ := inv.Flags.GetString("message")
			if msg == "" {
				return nil, newUsageError("--message is required")
			}
			return c.Projects.CreateThread(ctx, inv.Scope.BusinessID, inv.IntID(), msg)
		},
	},
	{
		Group: "projects", Verb: "add-thread-comment",
		Short:   "Add a comment to a project discussion message",
		Service: "Projects", Method: "AddThreadComment",
		Keys:  []string{"Projects/Add Comment to Project Discussion Message"},
		Class: ClassW, Scope: ScopeBusiness, HasID: true, IDName: "thread-id",
		ExtraFlags: func(fs *pflag.FlagSet) {
			fs.String("message", "", "the comment text (required)")
		},
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			msg, _ := inv.Flags.GetString("message")
			if msg == "" {
				return nil, newUsageError("--message is required")
			}
			return c.Projects.AddThreadComment(ctx, inv.Scope.BusinessID, inv.IntID(), msg)
		},
	},
}
