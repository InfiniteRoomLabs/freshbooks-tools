package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// imagesCommands wrap *freshbooks.ImagesService.
//
// images upload-without-account's lib method takes no account scope at
// all (ImagesService.UploadWithoutAccount(ctx, filename, r)), unlike
// docs/phases/4/commands.md's "S, --file <path>" row; the lib signature
// wins.
var imagesCommands = []Command{
	{
		Group: "images", Verb: "upload",
		Short:   "Upload a logo or proposal image",
		Service: "Images", Method: "Upload",
		Keys:  []string{"Uploader/Upload Logo or Proposal Image", "Invoices/Upload Logo/Upload Logo", "Expenses/Upload Expense Receipt Image/Upload Receipt Image"},
		Class: ClassW, Scope: ScopeAccount, Upload: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			f, name, err := inv.OpenUpload()
			if err != nil {
				return nil, err
			}
			defer f.Close() //nolint:errcheck // read-only upload source
			return c.Images.Upload(ctx, inv.Scope.AccountID, name, f)
		},
	},
	{
		Group: "images", Verb: "upload-without-account",
		Short:   "Upload an image with no account association",
		Service: "Images", Method: "UploadWithoutAccount",
		Keys:  []string{"Uploader/Upload Image Without AccountId", "Settings/Developer/Upload App Logo"},
		Class: ClassW, Upload: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			f, name, err := inv.OpenUpload()
			if err != nil {
				return nil, err
			}
			defer f.Close() //nolint:errcheck // read-only upload source
			return c.Images.UploadWithoutAccount(ctx, name, f)
		},
	},
}
