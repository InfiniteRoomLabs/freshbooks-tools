package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// specFor returns the registry entry named name, failing the test if none
// exists.
func specFor(t *testing.T, name string) Spec {
	t.Helper()
	for i := range All {
		if All[i].Name == name {
			return All[i]
		}
	}
	t.Fatalf("no such tool %q", name)
	return Spec{}
}

// mustMap type-asserts v to a map[string]any, failing the test if it is
// not one -- every schema this file builds arguments from has an object
// root.
func mustMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("value is not a map[string]any: %#v", v)
	}
	return m
}

// applicationFixture is a fixture server that answers every request with
// an Application carrying a live-looking client_secret, in whichever
// envelope the request's family and shape need.
func applicationFixture(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const secret = `super-secret-oauth-client-secret-do-not-leak`
		app := `{"client_id": "cid_123", "client_secret": "` + secret + `", "name": "Test App", "redirect_uri": "https://example.com/cb"}`
		switch {
		case strings.Contains(r.URL.Path, "partners/applications") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"response": [` + app + `]}`))
		default:
			_, _ = w.Write([]byte(`{"response": ` + app + `}`))
		}
	}))
}

// resultText renders both content shapes a CallToolResult carries, for a
// leak check that must not miss either one.
func resultText(t *testing.T, result *mcp.CallToolResult) (content, structured string) {
	t.Helper()
	content = errorContentText(result)
	if result.StructuredContent != nil {
		raw, err := json.Marshal(result.StructuredContent)
		if err != nil {
			t.Fatalf("marshaling StructuredContent: %v", err)
		}
		structured = string(raw)
	}
	return content, structured
}

// TestApplicationSecretRedacted is the written security constraint from
// docs/phases/3/plan.md: every tool returning freshbooks.Application
// zeroes ClientSecret before it reaches the result -- checked against
// both the Content (TextContent) block and StructuredContent, since the
// plan asked for absence "on the wire", not just on the Go struct.
func TestApplicationSecretRedacted(t *testing.T) {
	upstream := applicationFixture(t)
	defer upstream.Close()
	ctx := context.Background()
	clientSession := newTestSession(t, upstream, testScope, nil)

	for _, name := range []string{"identity_create_application", "identity_applications", "identity_update_application"} {
		t.Run(name, func(t *testing.T) {
			spec := specFor(t, name)
			args := synth(spec.InputSchema)
			result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if result.IsError {
				t.Fatalf("tool returned IsError: %s", errorContentText(result))
			}
			content, structured := resultText(t, result)
			for _, part := range []string{content, structured} {
				if strings.Contains(part, "super-secret-oauth-client-secret") {
					t.Fatalf("%s leaked client_secret: %s", name, part)
				}
				if strings.Contains(part, "client_secret") {
					t.Fatalf("%s: client_secret key survived (should be omitted, not just emptied): %s", name, part)
				}
			}
		})
	}
}

// sensitiveCase is one of the five tools registered with newSensitiveSpec
// (the four payment_options tokenization tools plus
// identity_update_application, which takes a required client_secret
// input -- see registry.go's newSensitiveSpec doc comment). mutateValid
// plants sensitive values into an otherwise well-typed, synth-generated
// argument set and returns them; mutateInvalid does the same but also
// corrupts one field's type, so the request fails schema validation --
// proving the sensitive values are absent from the generic "invalid
// arguments" message too (docs/phases/3/reports/security.md finding 1).
type sensitiveCase struct {
	tool          string
	mutateValid   func(args map[string]any) []string
	mutateInvalid func(args map[string]any) []string
}

var sensitiveCases = []sensitiveCase{
	{
		tool: "payment_options_fb_pay_tokenize",
		mutateValid: func(args map[string]any) []string {
			body := args["body"].(map[string]any) //nolint:errcheck // synth always builds an object here
			body["card_number"] = "4000000000000001"
			body["cvv"] = "111"
			return []string{"4000000000000001", "111"}
		},
		mutateInvalid: func(args map[string]any) []string {
			// The whole body as a bare string, where the schema wants an
			// object: exactly the "card as a string" shape
			// docs/phases/3/reports/security.md's finding 1 used as its
			// failure scenario.
			args["body"] = "4000000000000001 cvv 111"
			return []string{"4000000000000001", "111"}
		},
	},
	{
		tool: "payment_options_stripe_tokenize",
		mutateValid: func(args map[string]any) []string {
			body := args["body"].(map[string]any) //nolint:errcheck
			body["card_number"] = "4000000000000002"
			args["api_key"] = "pk_test_super_secret_stripe_key"
			return []string{"4000000000000002", "pk_test_super_secret_stripe_key"}
		},
		mutateInvalid: func(args map[string]any) []string {
			args["api_key"] = "pk_test_super_secret_stripe_key"
			// card_number as a bare number, where the schema wants a
			// string: the other failure scenario finding 1 named.
			args["body"] = map[string]any{"card_number": 4000000000000002}
			return []string{"pk_test_super_secret_stripe_key"}
		},
	},
	{
		tool: "payment_options_stripe_create_setup_intent",
		mutateValid: func(args map[string]any) []string {
			args["payment_method"] = "pm_super_secret_payment_method_ref"
			return []string{"pm_super_secret_payment_method_ref"}
		},
		mutateInvalid: func(args map[string]any) []string {
			args["payment_method"] = "pm_super_secret_payment_method_ref"
			args["account_id"] = 12345 // wrong type: number, not string
			return []string{"pm_super_secret_payment_method_ref"}
		},
	},
	{
		tool: "payment_options_save_credit_card",
		mutateValid: func(args map[string]any) []string {
			body := args["body"].(map[string]any) //nolint:errcheck
			tokens, _ := body["credit_card_tokens"].([]any)
			if len(tokens) > 0 {
				tok := tokens[0].(map[string]any) //nolint:errcheck
				tok["token"] = "tok_super_secret_saved_card_token"
			}
			return []string{"tok_super_secret_saved_card_token"}
		},
		mutateInvalid: func(args map[string]any) []string {
			args["body"] = "not-an-object"
			return nil
		},
	},
	{
		tool: "identity_update_application",
		mutateValid: func(args map[string]any) []string {
			body := args["body"].(map[string]any) //nolint:errcheck
			body["client_secret"] = "app_super_secret_oauth_client_secret"
			return []string{"app_super_secret_oauth_client_secret"}
		},
		mutateInvalid: func(args map[string]any) []string {
			// Missing every other required field on Body: still
			// schema-invalid (jsonschema-go validates nested required
			// properties too), and still carries the secret.
			args["body"] = map[string]any{"client_secret": "app_super_secret_oauth_client_secret"}
			return []string{"app_super_secret_oauth_client_secret"}
		},
	},
}

// TestSensitiveToolsNeverEchoInput is the written security constraint from
// docs/phases/3/plan.md, extended to cover the SDK schema-validation echo
// path docs/phases/3/reports/security.md finding 1 found: for every tool
// registered with newSensitiveSpec, neither a well-typed call nor a
// schema-invalid one lets a sensitive input value reach the result
// (Content or StructuredContent) or a debug-level log.
func TestSensitiveToolsNeverEchoInput(t *testing.T) {
	for _, tc := range sensitiveCases {
		t.Run(tc.tool, func(t *testing.T) {
			spec := specFor(t, tc.tool)

			t.Run("well-typed", func(t *testing.T) {
				var logBuf bytes.Buffer
				logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				upstream, _ := fakeUpstream(t)
				defer upstream.Close()
				session := newTestSession(t, upstream, testScope, logger)

				args := mustMap(t, synth(spec.InputSchema))
				sensitive := tc.mutateValid(args)

				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
				if err != nil {
					t.Fatalf("CallTool: %v", err)
				}
				assertNoLeak(t, result, logBuf.String(), sensitive)
			})

			t.Run("schema-invalid", func(t *testing.T) {
				var logBuf bytes.Buffer
				logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				upstream, _ := fakeUpstream(t)
				defer upstream.Close()
				session := newTestSession(t, upstream, testScope, logger)

				args := mustMap(t, synth(spec.InputSchema))
				sensitive := tc.mutateInvalid(args)

				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: args})
				if err != nil {
					t.Fatalf("CallTool: %v", err)
				}
				if !result.IsError {
					t.Fatalf("want IsError for schema-invalid input")
				}
				assertNoLeak(t, result, logBuf.String(), sensitive)
			})
		})
	}
}

// assertNoLeak fails the test if any sensitive value appears in the
// result's Content, its StructuredContent, or the captured log output.
func assertNoLeak(t *testing.T, result *mcp.CallToolResult, logs string, sensitive []string) {
	t.Helper()
	content, structured := resultText(t, result)
	for _, s := range sensitive {
		if s == "" {
			continue
		}
		if strings.Contains(content, s) {
			t.Errorf("Content leaked %q: %s", s, content)
		}
		if strings.Contains(structured, s) {
			t.Errorf("StructuredContent leaked %q: %s", s, structured)
		}
		if strings.Contains(logs, s) {
			t.Errorf("log output leaked %q: %s", s, logs)
		}
	}
}
