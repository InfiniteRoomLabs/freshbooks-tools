// Package tools holds one MCP tool per freshbooks client-library method,
// named {service_field_snake}_{method_snake} (for example invoices_list):
// 168 tools, one per exported service method minus the 17 All iterators
// and Authorization/Revoke Refresh Token (the MCP is a token consumer, not
// an owner). The registry is data-driven -- see registry.go's Spec and
// newSpec -- not 168 hand-rolled handlers. See docs/mcp.md and the
// inventory parity contract in freshbooks/internal/inventory.
package tools
