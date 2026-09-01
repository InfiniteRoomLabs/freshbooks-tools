package freshbooks

import (
	"context"
	"io"
	"net/http"
)

// UploadedImage identifies an image FreshBooks has stored. JWT is the token
// other resources (e.g. an invoice profile's logo, a proposal's cover
// image) reference to attach it; Link is a direct URL to the stored image.
type UploadedImage struct {
	// JWT identifies the upload for attaching it to another resource.
	JWT string `json:"jwt"`
	// PublicID is a public-facing identifier for the upload, generally
	// equal to JWT.
	PublicID string `json:"public_id"`
	// Filename is the storage filename FreshBooks assigned.
	Filename string `json:"filename"`
	// Link is a direct URL to the stored image. The API returns it as a
	// sibling of the "image" object, not nested inside it; Upload and
	// UploadWithoutAccount copy it in after decoding.
	Link string `json:"-"`
}

// imageUploadResponse is the {"image": {...}, "link": "..."} shape both
// upload endpoints answer with: link sits beside image, not inside it.
type imageUploadResponse struct {
	Image UploadedImage `json:"image"`
	Link  string        `json:"link"`
}

func (r imageUploadResponse) result() *UploadedImage {
	img := r.Image
	img.Link = r.Link
	return &img
}

// Upload sends r's contents (up to the transport's upload size bound) to
// acct's image store under filename, returning the stored image's
// reference.
//
// inventory: Uploader/Upload Logo or Proposal Image
// inventory: Invoices/Upload Logo/Upload Logo
// inventory: Expenses/Upload Expense Receipt Image/Upload Receipt Image
func (s *ImagesService) Upload(ctx context.Context, acct AccountID, filename string, r io.Reader) (*UploadedImage, error) {
	if err := pathSegment(string(acct)); err != nil {
		return nil, err
	}
	path := "/uploads/account/" + string(acct) + "/images"
	var resp imageUploadResponse
	if err := s.client.doMultipart(ctx, http.MethodPost, path, FamilyBusiness, "content", filename, r, &resp); err != nil {
		return nil, err
	}
	return resp.result(), nil
}

// UploadWithoutAccount uploads an image with no account scope, used for
// account-independent assets such as a developer app's logo.
//
// inventory: Uploader/Upload Image Without AccountId
// inventory: Settings/Developer/Upload App Logo
func (s *ImagesService) UploadWithoutAccount(ctx context.Context, filename string, r io.Reader) (*UploadedImage, error) {
	const path = "/uploads/images"
	var resp imageUploadResponse
	if err := s.client.doMultipart(ctx, http.MethodPost, path, FamilyBusiness, "content", filename, r, &resp); err != nil {
		return nil, err
	}
	return resp.result(), nil
}
