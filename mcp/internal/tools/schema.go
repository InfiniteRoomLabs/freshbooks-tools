package tools

import (
	"reflect"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/google/jsonschema-go/jsonschema"
)

// typeOverrides fixes the three lib types whose default reflected schema
// go-sdk's jsonschema.ForType cannot produce (verified 169/169 lib
// request/option/result types pass with these three overrides in place):
//
//   - Date and DateTime both embed time.Time, and jsonschema-go refuses a
//     custom schema on an embedded struct field unless it has type
//     "object" ("custom schema for embedded struct must have type
//     object, got string") -- so instead of overriding the embedded
//     time.Time, the override targets Date/DateTime themselves, which
//     bypasses struct-field reflection entirely (jsonschema.ForType
//     checks its TypeSchemas map before it ever looks at a type's
//     Kind()).
//   - ProfitLossLine is self-referential (a line can nest sub-lines), and
//     jsonschema.ForType rejects the cycle ("cycle detected for type").
//
// See docs/phases/3/plan.md decision D3.
var typeOverrides = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[freshbooks.Date]():           {Type: "string", Format: "date"},
	reflect.TypeFor[freshbooks.DateTime]():       {Type: "string"},
	reflect.TypeFor[freshbooks.ProfitLossLine](): {Type: "object", AdditionalProperties: &jsonschema.Schema{}},
}

// schemaFor builds In's input schema. Each call site (newSpec, once per
// tool at package init) invokes this exactly once; the resulting *Schema
// is then reused for the lifetime of the process across every *mcp.Server
// Register builds, so a shared mcp.SchemaCache (wired in
// internal/server) resolves it once too, never per request.
func schemaFor[In any]() *jsonschema.Schema {
	schema, err := jsonschema.ForType(reflect.TypeFor[In](), &jsonschema.ForOptions{TypeSchemas: typeOverrides})
	if err != nil {
		panic("freshbooks-mcp/tools: computing input schema: " + err.Error())
	}
	return schema
}
