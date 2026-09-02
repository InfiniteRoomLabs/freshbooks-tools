package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// billVendorsCommands wrap *freshbooks.BillVendorsService.
var billVendorsCommands = []Command{
	{
		Group: "bill-vendors", Verb: "list",
		Short:   "List a business's bill vendors",
		Service: "BillVendors", Method: "List",
		Keys:  []string{"Expenses/Vendors (Beta)/Get Vendors"},
		Class: ClassRO, Scope: ScopeAccount, List: true, HasAll: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			opts := &freshbooks.BillVendorListOptions{Search: inv.Search(), Page: inv.Page(), PerPage: inv.PerPage()}
			if inv.All() {
				return collectAll(c.BillVendors.All(ctx, inv.Scope.AccountID, opts))
			}
			return c.BillVendors.List(ctx, inv.Scope.AccountID, opts)
		},
	},
	{
		Group: "bill-vendors", Verb: "create",
		Short:   "Add a bill vendor",
		Service: "BillVendors", Method: "Create",
		Keys:  []string{"Expenses/Vendors (Beta)/Add Vendor"},
		Class: ClassW, Scope: ScopeAccount, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.BillVendorRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.BillVendors.Create(ctx, inv.Scope.AccountID, &body)
		},
	},
	{
		Group: "bill-vendors", Verb: "update",
		Short:   "Update a bill vendor's details",
		Service: "BillVendors", Method: "Update",
		Keys:  []string{"Expenses/Vendors (Beta)/Edit Vendor Details"},
		Class: ClassI, Scope: ScopeAccount, HasID: true, Body: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			var body freshbooks.BillVendorRequest
			if err := inv.DecodeBody(&body); err != nil {
				return nil, err
			}
			return c.BillVendors.Update(ctx, inv.Scope.AccountID, inv.IntID(), &body)
		},
	},
	{
		Group: "bill-vendors", Verb: "delete",
		Short:   "Delete a bill vendor",
		Service: "BillVendors", Method: "Delete",
		Keys:  []string{"Expenses/Vendors (Beta)/Delete Vendor"},
		Class: ClassD, Scope: ScopeAccount, HasID: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			return void(c.BillVendors.Delete(ctx, inv.Scope.AccountID, inv.IntID()))
		},
	},
}
