package tools

import (
	"encoding/json"
	"errors"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// errResult builds an IsError CallToolResult carrying text as its sole
// content block. Every tool error path returns one of these as the
// handler's first (*mcp.CallToolResult) return value with a nil error, so
// the SDK's own error wrapping in mcp.AddTool never runs: a Go error
// returned from a tool handler becomes a JSON-RPC protocol error, which
// would hide an API failure from the model instead of reporting it.
func errResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// fbErrorContent is the documented JSON shape of a mapped *freshbooks.Error.
type fbErrorContent struct {
	Status  int    `json:"status"`
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Family  string `json:"family,omitempty"`
}

// errorText renders err for a tool's IsError content. A *freshbooks.Error
// becomes the documented {status, code, message, field, family} JSON; any
// other error becomes its Error() text.
//
// Neither branch formats a tool's input struct, and the lib's own *Error
// never carries a raw request body or bearer token in Message (see
// freshbooks/errors.go), so this function cannot leak the tokenization or
// application-secret constraints' protected fields into an error result.
func errorText(err error) string {
	var fbErr *freshbooks.Error
	if errors.As(err, &fbErr) {
		body, mErr := json.Marshal(fbErrorContent{
			Status:  fbErr.StatusCode,
			Code:    fbErr.Code,
			Message: fbErr.Message,
			Field:   fbErr.Field,
			Family:  string(fbErr.Family),
		})
		if mErr == nil {
			return string(body)
		}
	}
	return err.Error()
}
