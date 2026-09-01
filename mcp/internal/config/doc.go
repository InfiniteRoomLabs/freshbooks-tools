// Package config resolves freshbooks-mcp's configuration: cobra flags with
// FRESHBOOKS_MCP_* environment twins (flag takes precedence over env, env
// over a built-in default), no config file, plus the lib-wide token and
// default-scope environment (FRESHBOOKS_ACCESS_TOKEN and friends -- not
// FRESHBOOKS_MCP_*; see docs/mcp.md). Every secret Config carries
// (AccessToken, ClientSecret, RefreshToken) is redacted from String() and
// LogValue(), so logging or printing a Config cannot leak one.
package config
