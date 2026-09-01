package tools

import (
	"context"
	"sort"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Call is the shape of a tool's implementation: given the resolved scope
// and the decoded input, it invokes exactly one freshbooks client-library
// method and returns its result. Call implementations never build a
// CallToolResult themselves and never format their In value with %v or
// %+v; the generic dispatcher newSpec builds handles both the scope
// resolution and the error mapping.
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

// newSpec builds one Spec. It is called exactly once per tool, from a
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
					return errResult("missing required scope: " + missing + " (set it on the request, or configure a server default for it)"), nil, nil
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
// Token (see docs/phases/3/plan.md decisions D1-D2). 168 entries.
var All = buildRegistry()

func buildRegistry() []Spec {
	var r []Spec
	r = append(r, attachmentsSpecs...)
	r = append(r, billPaymentsSpecs...)
	r = append(r, billsSpecs...)
	r = append(r, billVendorsSpecs...)
	r = append(r, callbacksSpecs...)
	r = append(r, clientsSpecs...)
	r = append(r, contactsSpecs...)
	r = append(r, creditNotesSpecs...)
	r = append(r, estimatesSpecs...)
	r = append(r, expenseCategoriesSpecs...)
	r = append(r, expensesSpecs...)
	r = append(r, gatewaysSpecs...)
	r = append(r, identitySpecs...)
	r = append(r, imagesSpecs...)
	r = append(r, invoiceProfilesSpecs...)
	r = append(r, invoicesSpecs...)
	r = append(r, itemsSpecs...)
	r = append(r, journalSpecs...)
	r = append(r, ledgerAccountsSpecs...)
	r = append(r, otherIncomeSpecs...)
	r = append(r, paymentOptionsSpecs...)
	r = append(r, paymentsSpecs...)
	r = append(r, projectsSpecs...)
	r = append(r, reportsSpecs...)
	r = append(r, retainersSpecs...)
	r = append(r, serviceRatesSpecs...)
	r = append(r, servicesSpecs...)
	r = append(r, staffSpecs...)
	r = append(r, systemsSpecs...)
	r = append(r, tasksSpecs...)
	r = append(r, taxesSpecs...)
	r = append(r, teamMembersSpecs...)
	r = append(r, timeEntriesSpecs...)
	return r
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
