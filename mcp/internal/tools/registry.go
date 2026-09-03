package tools

import (
	"context"
	"encoding/json"
	"slices"
	"sort"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Call is the shape of a tool's implementation: given the resolved scope
// and the decoded input, it invokes exactly one freshbooks client-library
// method and returns its result. Call implementations never build a
// CallToolResult themselves and never format their In value with %v or
// %+v; the generic dispatchers newSpec and newSensitiveSpec build handle
// both the scope resolution and the error mapping.
type Call[In any] func(ctx context.Context, c *freshbooks.Client, scope Scope, in In) (any, error)

// Spec is one MCP tool registration: everything Register needs to add the
// tool to a server, and everything the parity test needs to check it
// against the lib and the inventory.
type Spec struct {
	// Name is the tool name, {service_field_snake}_{method_snake}.
	Name string
	// Description is one sentence plus the FreshBooks docs URL.
	Description string
	// Service is the *freshbooks.Client field name the tool wraps, e.g.
	// "Clients". The parity test reconciles this against reflection over
	// *freshbooks.Client's exported service fields.
	Service string
	// Method is the exported method name on Service, e.g. "List".
	Method string
	// Keys are the lib method's // inventory: keys, stacked exactly as
	// freshbooks carries them (see identity.go:86-87 for the pattern).
	// identity_whoami carries none: it is a lib convenience with no
	// Postman-collection entry of its own.
	Keys []string
	// Annotations are the tool's MCP hints (ReadOnlyHint, DestructiveHint,
	// IdempotentHint, OpenWorldHint).
	Annotations *mcp.ToolAnnotations
	// InputSchema is precomputed once at package init by schemaFor; see
	// its doc comment for why that matters.
	InputSchema *jsonschema.Schema

	add func(server *mcp.Server, client *freshbooks.Client, defaults Scope)
}

// missingScopeResult builds the IsError result the plan's "named missing
// field" contract requires.
func missingScopeResult(field string) *mcp.CallToolResult {
	return errResult("missing required scope: " + field + " (set it on the request, or configure a server default for it)")
}

// successResult builds a successful CallToolResult from a Call's returned
// value, mirroring what mcp.AddTool's generic ToolHandlerFor path does for
// Out = any: StructuredContent carries the marshaled value, and Content
// falls back to the same JSON as one TextContent block (go-sdk
// mcp/server.go's toolForErr).
func successResult(out any) *mcp.CallToolResult {
	if out == nil {
		return &mcp.CallToolResult{}
	}
	outBytes, err := json.Marshal(out)
	if err != nil {
		// Every Call closure returns a lib value that always marshals; a
		// failure here is an internal bug, not a caller-input problem.
		return errResult("internal error marshaling the result")
	}
	raw := json.RawMessage(outBytes)
	return &mcp.CallToolResult{
		StructuredContent: raw,
		Content:           []mcp.Content{&mcp.TextContent{Text: string(outBytes)}},
	}
}

// newSpec builds one Spec whose tool is registered through mcp.AddTool's
// generic dispatcher. It is called exactly once per tool, from a
// package-level var declaration in the tool's resource file, so
// schemaFor's reflection runs once at package init regardless of how many
// times Register later adds the tool to a server.
func newSpec[In any](name, description, service, method string, keys []string, annot *mcp.ToolAnnotations, call Call[In]) Spec {
	schema := schemaFor[In]()
	return Spec{
		Name:        name,
		Description: description,
		Service:     service,
		Method:      method,
		Keys:        keys,
		Annotations: annot,
		InputSchema: schema,
		add: func(server *mcp.Server, client *freshbooks.Client, defaults Scope) {
			tool := &mcp.Tool{
				Name:        name,
				Description: description,
				Annotations: annot,
				InputSchema: schema,
			}
			mcp.AddTool(server, tool, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
				scope, missing := resolveScope(in, defaults)
				if missing != "" {
					return missingScopeResult(missing), nil, nil
				}
				out, err := call(ctx, client, scope, in)
				if err != nil {
					return errResult(errorText(err)), nil, nil
				}
				return nil, out, nil
			})
		},
	}
}

// newSensitiveSpec is newSpec for a tool whose input can carry data that
// must never be echoed back to a caller: the four payment_options
// tokenization tools and identity_update_application, all of which take a
// required string field (a card number, a CVV, a Stripe key, or an OAuth
// client secret) with no way to mark it write-only in JSON Schema.
//
// mcp.AddTool's generic dispatcher validates and unmarshals arguments
// *before* any handler runs, and on failure quotes the offending value
// straight into the tool result (go-sdk mcp/server.go's applySchema call,
// jsonschema-go validate.go's "type: %v has type %q, want %q" and
// sibling messages) -- a real echo path the written "never echo"
// constraint in docs/phases/3/plan.md did not anticipate and this
// module's own error mapping in errors.go cannot intercept, because it
// never runs. See docs/phases/3/reports/security.md finding 1.
//
// newSensitiveSpec sidesteps it entirely: it resolves the same
// precomputed schema once at init and registers through the untyped
// (*mcp.Server).AddTool with a hand-written ToolHandler that validates
// and decodes arguments itself, and on ANY validation or decode failure
// returns one generic, name-only message with nothing from the input
// interpolated. A well-typed call behaves identically to newSpec's path;
// only the failure mode differs.
func newSensitiveSpec[In any](name, description, service, method string, keys []string, annot *mcp.ToolAnnotations, call Call[In]) Spec {
	schema := schemaFor[In]()
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		panic("freshbooks-mcp/tools: resolving schema for " + name + ": " + err.Error())
	}
	invalidArgs := errResult("invalid arguments for " + name + ": the input did not match the tool's input schema")

	return Spec{
		Name:        name,
		Description: description,
		Service:     service,
		Method:      method,
		Keys:        keys,
		Annotations: annot,
		InputSchema: schema,
		add: func(server *mcp.Server, client *freshbooks.Client, defaults Scope) {
			tool := &mcp.Tool{
				Name:        name,
				Description: description,
				Annotations: annot,
				InputSchema: schema,
			}
			server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				raw := req.Params.Arguments
				if len(raw) == 0 {
					raw = json.RawMessage("{}")
				}

				var probe map[string]any
				if jsonErr := json.Unmarshal(raw, &probe); jsonErr != nil {
					return invalidArgs, nil
				}
				if valErr := resolved.Validate(probe); valErr != nil {
					return invalidArgs, nil
				}

				var in In
				if jsonErr := json.Unmarshal(raw, &in); jsonErr != nil {
					return invalidArgs, nil
				}

				scope, missing := resolveScope(in, defaults)
				if missing != "" {
					return missingScopeResult(missing), nil
				}
				out, callErr := call(ctx, client, scope, in)
				if callErr != nil {
					return errResult(errorText(callErr)), nil
				}
				return successResult(out), nil
			})
		},
	}
}

// Register adds every tool in the registry to server, bound to client and
// falling back to defaults for any scope field a call omits.
func Register(server *mcp.Server, client *freshbooks.Client, defaults Scope) {
	for _, s := range All {
		s.add(server, client, defaults)
	}
}

// Manifest returns every tool's public shape, sorted by name: what the
// `tools` command prints and what docs generation reads.
func Manifest() []*mcp.Tool {
	out := make([]*mcp.Tool, len(All))
	for i, s := range All {
		out[i] = &mcp.Tool{
			Name:        s.Name,
			Description: s.Description,
			Annotations: s.Annotations,
			InputSchema: s.InputSchema,
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// All is the full tool registry: one Spec per freshbooks client-library
// method, minus the 17 All iterators and Authorization/Revoke Refresh
// Token (see docs/phases/3/plan.md decisions D1-D2). 169 entries (168
// from Phase 3, plus Phase 8's keyless time_entries_list_with_totals).
var All = buildRegistry()

func buildRegistry() []Spec {
	return slices.Concat(
		attachmentsSpecs, billPaymentsSpecs, billsSpecs, billVendorsSpecs,
		callbacksSpecs, clientsSpecs, contactsSpecs, creditNotesSpecs,
		estimatesSpecs, expenseCategoriesSpecs, expensesSpecs, gatewaysSpecs,
		identitySpecs, imagesSpecs, invoiceProfilesSpecs, invoicesSpecs,
		itemsSpecs, journalSpecs, ledgerAccountsSpecs, otherIncomeSpecs,
		paymentOptionsSpecs, paymentsSpecs, projectsSpecs, reportsSpecs,
		retainersSpecs, serviceRatesSpecs, servicesSpecs, staffSpecs,
		systemsSpecs, tasksSpecs, taxesSpecs, teamMembersSpecs, timeEntriesSpecs,
	)
}

// Annotation hint sets, one per column tools.md documents (RO/D/I/W).
// OpenWorldHint is true for every tool: FreshBooks is an external service,
// never a closed in-memory domain.
var (
	hintRO = &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(true)}
	hintD  = &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(true)}
	hintI  = &mcp.ToolAnnotations{IdempotentHint: true, OpenWorldHint: boolPtr(true)}
	hintW  = &mcp.ToolAnnotations{OpenWorldHint: boolPtr(true)}
)

func boolPtr(b bool) *bool { return &b }
