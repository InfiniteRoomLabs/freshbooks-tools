package tools

import (
	"context"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

type attachmentsUploadExpenseReceiptIn struct {
	AcctScope
	uploadIn
}

// attachmentsSpecs are the tools wrapping *freshbooks.AttachmentsService.
var attachmentsSpecs = []Spec{
	newSpec("attachments_upload_expense_receipt",
		"Upload a receipt image or PDF for later attachment to an expense. See https://www.freshbooks.com/api/expenses.",
		"Attachments", "UploadExpenseReceipt",
		[]string{"Uploader/Upload Expense Receipt"}, hintW,
		func(ctx context.Context, c *freshbooks.Client, scope Scope, in attachmentsUploadExpenseReceiptIn) (any, error) {
			r, err := in.reader()
			if err != nil {
				return nil, err
			}
			return c.Attachments.UploadExpenseReceipt(ctx, scope.AccountID, in.Filename, r)
		}),
}
