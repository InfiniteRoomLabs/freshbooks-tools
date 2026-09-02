package cmd

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// attachmentsCommands wrap *freshbooks.AttachmentsService.
var attachmentsCommands = []Command{
	{
		Group: "attachments", Verb: "upload-expense-receipt",
		Short:   "Upload a receipt image for an expense",
		Service: "Attachments", Method: "UploadExpenseReceipt",
		Keys:  []string{"Uploader/Upload Expense Receipt"},
		Class: ClassW, Scope: ScopeAccount, Upload: true,
		Run: func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error) {
			f, name, err := inv.OpenUpload()
			if err != nil {
				return nil, err
			}
			defer f.Close() //nolint:errcheck // read-only upload source
			return c.Attachments.UploadExpenseReceipt(ctx, inv.Scope.AccountID, name, f)
		},
	},
}
