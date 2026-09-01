package freshbooks

import (
	"context"
	"io"
	"net/http"
)

// UploadedAttachment identifies a file FreshBooks has stored for use as an
// expense receipt. JWT is the token an expense's Attachment field
// references to attach it.
//
// The full field list ("filename", "public_id", "jwt", "media_type",
// "uuid") is confirmed from the FreshBooks expense-attachments docs page;
// the Postman collection carries no example response for this endpoint.
type UploadedAttachment struct {
	// JWT identifies the upload for attaching it to an expense.
	JWT string `json:"jwt"`
	// PublicID is a public-facing identifier for the upload.
	PublicID string `json:"public_id"`
	// Filename is the storage filename FreshBooks assigned.
	Filename string `json:"filename"`
	// MediaType is the uploaded file's media type, e.g. "image/jpeg".
	MediaType string `json:"media_type"`
	// UUID identifies the upload.
	UUID string `json:"uuid"`
}

// UploadExpenseReceipt sends r's contents (up to the transport's upload
// size bound) as an expense receipt, returning the stored attachment's
// reference. Pass the returned JWT and MediaType in an expense's Attachment
// field to include the receipt on that expense.
//
// inventory: Uploader/Upload Expense Receipt
func (s *AttachmentsService) UploadExpenseReceipt(ctx context.Context, acct AccountID, filename string, r io.Reader) (*UploadedAttachment, error) {
	if err := pathSegment(string(acct)); err != nil {
		return nil, err
	}
	path := "/uploads/account/" + string(acct) + "/attachments"
	var resp struct {
		Attachment UploadedAttachment `json:"attachment"`
	}
	if err := s.client.doMultipart(ctx, http.MethodPost, path, FamilyBusiness, "content", filename, r, &resp); err != nil {
		return nil, err
	}
	return &resp.Attachment, nil
}
