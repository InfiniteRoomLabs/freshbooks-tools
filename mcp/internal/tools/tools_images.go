package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type imagesUploadIn struct {
	AcctScope
	uploadIn
}

// imagesSpecs are the tools wrapping *freshbooks.ImagesService.
var imagesSpecs = []Spec{
	newSpec("images_upload",
		"Upload a logo, proposal image, or receipt image for an account. See https://www.freshbooks.com/api/estimates.",
		"Images", "Upload",
		[]string{"Uploader/Upload Logo or Proposal Image", "Invoices/Upload Logo/Upload Logo", "Expenses/Upload Expense Receipt Image/Upload Receipt Image"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in imagesUploadIn) (any, error) {
			r, err := in.reader()
			if err != nil {
				return nil, err
			}
			return c.Images.Upload(ctx, scope.AccountID, in.Filename, r)
		}),
	newSpec("images_upload_without_account",
		"Upload an image with no account association, e.g. an app logo. See https://www.freshbooks.com/api/estimates.",
		"Images", "UploadWithoutAccount",
		[]string{"Uploader/Upload Image Without AccountId", "Settings/Developer/Upload App Logo"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in uploadIn) (any, error) {
			r, err := in.reader()
			if err != nil {
				return nil, err
			}
			return c.Images.UploadWithoutAccount(ctx, in.Filename, r)
		}),
}
