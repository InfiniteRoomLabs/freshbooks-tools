package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type billVendorsListIn struct {
	AcctScope
	listIn
}

type billVendorsCreateIn struct {
	AcctScope
	Body freshbooks.BillVendorRequest `json:"body" jsonschema:"the vendor fields to create"`
}

type billVendorsUpdateIn struct {
	AcctScope
	idIn
	Body freshbooks.BillVendorRequest `json:"body" jsonschema:"the vendor fields to update"`
}

type billVendorsIDIn struct {
	AcctScope
	idIn
}

// billVendorsSpecs are the tools wrapping *freshbooks.BillVendorsService.
var billVendorsSpecs = []Spec{
	newSpec("bill_vendors_list",
		"List a business's bill vendors. See https://www.freshbooks.com/api/bills.",
		"BillVendors", "List",
		[]string{"Expenses/Vendors (Beta)/Get Vendors"}, hintRO,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billVendorsListIn) (any, error) {
			return c.BillVendors.List(ctx, scope.AccountID, &freshbooks.BillVendorListOptions{Search: in.search(), Page: in.Page, PerPage: in.PerPage})
		}),
	newSpec("bill_vendors_create",
		"Add a bill vendor. See https://www.freshbooks.com/api/bills.",
		"BillVendors", "Create",
		[]string{"Expenses/Vendors (Beta)/Add Vendor"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billVendorsCreateIn) (any, error) {
			return c.BillVendors.Create(ctx, scope.AccountID, &in.Body)
		}),
	newSpec("bill_vendors_update",
		"Update a bill vendor's details. See https://www.freshbooks.com/api/bills.",
		"BillVendors", "Update",
		[]string{"Expenses/Vendors (Beta)/Edit Vendor Details"}, hintI,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billVendorsUpdateIn) (any, error) {
			return c.BillVendors.Update(ctx, scope.AccountID, in.ID, &in.Body)
		}),
	newSpec("bill_vendors_delete",
		"Delete a bill vendor. See https://www.freshbooks.com/api/bills.",
		"BillVendors", "Delete",
		[]string{"Expenses/Vendors (Beta)/Delete Vendor"}, hintD,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in billVendorsIDIn) (any, error) {
			return void(c.BillVendors.Delete(ctx, scope.AccountID, in.ID))
		}),
}
