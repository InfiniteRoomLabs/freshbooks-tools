package tools

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
)

// uploadIn is embedded by every file-upload tool input: a filename and the
// file's content, base64-encoded (MCP tool arguments are JSON; there is no
// binary transport).
type uploadIn struct {
	Filename      string `json:"filename" jsonschema:"the file name, with extension"`
	ContentBase64 string `json:"content_base64" jsonschema:"the file's content, base64-encoded"`
}

// reader decodes ContentBase64 into an io.Reader for the lib's upload
// methods, which all take (filename string, r io.Reader).
func (u uploadIn) reader() (io.Reader, error) {
	data, err := base64.StdEncoding.DecodeString(u.ContentBase64)
	if err != nil {
		return nil, fmt.Errorf("content_base64 is not valid base64: %w", err)
	}
	return bytes.NewReader(data), nil
}

// binaryResult is the output shape for every tool whose lib method returns
// raw bytes (Invoices.PDF, Reports.DownloadInvoiceDetailsCSV): base64
// content plus enough metadata for a caller to know what it received.
type binaryResult struct {
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	ContentB64  string `json:"content_base64"`
}

func newBinaryResult(contentType string, data []byte) *binaryResult {
	return &binaryResult{
		ContentType: contentType,
		Size:        len(data),
		ContentB64:  base64.StdEncoding.EncodeToString(data),
	}
}
