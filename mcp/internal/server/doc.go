// Package server holds the stateless MCP server construction for
// freshbooks-mcp: the stdio transport (one lib client for the life of the
// process) and the streamable-HTTP transport (mcp.StreamableHTTPOptions{
// Stateless: true}, a fresh per-request lib client authenticated with that
// request's bearer token). Both share one mcp.SchemaCache so a tool's
// input schema is resolved once, not per request. See docs/mcp.md.
package server
