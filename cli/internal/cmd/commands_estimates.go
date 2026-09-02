package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// estimatesCommands wrap *freshbooks.EstimatesService.
var estimatesCommands = []Command{
	{
		Group: "estimates", Verb: "list",
		Short:   "List estimates",
		Service: "Estimates", Method: "List",
		Keys:  []string{"Estimates/List Estimates"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasInclude: true, HasAll: true, HasSort: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.EstimateListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage(), Include: inv.Include()}
			if inv.All() {
				return collectAll(c.Estimates.All(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...))
			}
			return c.Estimates.List(ctx, inv.Scope.AccountID, opts, inv.SortOpt()...)
		},
	},
	{
		Group: "estimates", Verb: "get",
		Short:   "Get a single estimate",
		Service: "Estimates", Method: "Get",
		Keys:  []string{"Estimates/Single Estimate"},
		Class: ClassRO, Scope: ScopeAccount, HasID: true, HasInclude: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Estimates.Get(ctx, inv.Scope.AccountID, inv.IntID(), inv.IncludeOpt()...)
		},
	},
	{
		Group: "estimates", Verb: "create",
		Short:   "Create an estimate",
		Service: "Estimates", Method: "Create",
		Keys:  []string{"Estimates/Create Single Proposal w/ Sections, Logos, and E-signature", "Estimates/Single Estimate With Estimate Lines"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.EstimateWriteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Estimates.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "estimates", Verb: "update",
		Short:   "Update an estimate",
		Service: "Estimates", Method: "Update",
		Keys:  []string{"Estimates/Update Estimate"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.EstimateWriteRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.Estimates.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "estimates", Verb: "delete",
		Short:   "Delete an estimate",
		Service: "Estimates", Method: "Delete",
		Keys:  []string{"Estimates/Delete Estimate"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.Estimates.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
	{
		Group: "estimates", Verb: "accept",
		Short:   "Accept an estimate",
		Service: "Estimates", Method: "Accept",
		Keys:  []string{"Estimates/Accept Estimate"},
		Class: ClassI, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return c.Estimates.Accept(ctx, inv.Scope.AccountID, inv.IntID())
		},
	},
	{
		Group: "estimates", Verb: "send",
		Short:   "Send an estimate by email",
		Service: "Estimates", Method: "Send",
		Keys:  []string{"Estimates/Send Estimate by Email"},
		Class: ClassW, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.EstimateSendRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return void(c.Estimates.Send(ctx, inv.Scope.AccountID, inv.IntID(), &body))
		},
	},
}
