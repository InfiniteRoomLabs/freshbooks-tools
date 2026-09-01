package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestErrorShapeEndToEnd drives a real non-2xx response from the lib all
// the way through the MCP session, proving errors.go's documented
// {status, code, message, field, family} shape survives the whole
// lib -> Call -> newSpec dispatcher path -- not just errorText called
// directly on a hand-built *freshbooks.Error (docs/phases/3/reports/
// code-review.md finding 6).
func TestErrorShapeEndToEnd(t *testing.T) {
	ctx := context.Background()

	t.Run("422 with a field error", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"response":{"errors":[{"errno":1012,"field":"email","message":"Invalid email address","object":"client"}]}}`))
		}))
		defer upstream.Close()
		session := newTestSession(t, upstream, testScope, nil)

		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "clients_get", Arguments: map[string]any{"id": 1}})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !result.IsError {
			t.Fatal("want IsError for a 422 response")
		}

		var shape struct {
			Status  int    `json:"status"`
			Code    int    `json:"code"`
			Message string `json:"message"`
			Field   string `json:"field"`
			Family  string `json:"family"`
		}
		if err := json.Unmarshal([]byte(errorContentText(result)), &shape); err != nil {
			t.Fatalf("error content is not the documented JSON shape: %v; content = %s", err, errorContentText(result))
		}
		if shape.Status != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want %d", shape.Status, http.StatusUnprocessableEntity)
		}
		if shape.Code != 1012 {
			t.Errorf("code = %d, want 1012", shape.Code)
		}
		if shape.Field != "email" {
			t.Errorf("field = %q, want email", shape.Field)
		}
		if shape.Message == "" {
			t.Error("message is empty")
		}
		if shape.Family != "accounting" {
			t.Errorf("family = %q, want accounting", shape.Family)
		}
	})

	t.Run("401 unauthorized", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_token","error_description":"the access token is expired"}`))
		}))
		defer upstream.Close()
		session := newTestSession(t, upstream, testScope, nil)

		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "identity_whoami", Arguments: map[string]any{}})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if !result.IsError {
			t.Fatal("want IsError for a 401 response")
		}

		var shape struct {
			Status  int    `json:"status"`
			Message string `json:"message"`
			Family  string `json:"family"`
		}
		if err := json.Unmarshal([]byte(errorContentText(result)), &shape); err != nil {
			t.Fatalf("error content is not the documented JSON shape: %v; content = %s", err, errorContentText(result))
		}
		if shape.Status != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", shape.Status, http.StatusUnauthorized)
		}
		if shape.Message == "" {
			t.Error("message is empty")
		}
		if shape.Family != "auth" {
			t.Errorf("family = %q, want auth", shape.Family)
		}
	})
}
