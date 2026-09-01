package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
	"github.com/spf13/cobra"
)

// Config is freshbooks-mcp's fully resolved configuration.
type Config struct {
	// Transport is "stdio" or "http".
	Transport string
	// Addr is the address the http transport listens on.
	Addr string
	// Path is the URL path the http transport serves the MCP endpoint on.
	Path string
	// LogLevel is "debug", "info", "warn", or "error".
	LogLevel string
	// LogFormat is "json" or "text". Logs always go to stderr: stdout is
	// the stdio transport.
	LogFormat string

	// AccessToken, when set, is used as-is: a static bearer with no
	// refresh. Stdio mode only; http mode's bearer comes from each
	// request's Authorization header.
	AccessToken string
	// ClientID, ClientSecret, and TokenFile together select the rotating
	// token path: a lib auth.FileStore plus auth.NewTokenSource. Stdio
	// mode only.
	ClientID     string
	ClientSecret string
	TokenFile    string
	// RefreshToken seeds TokenFile's store the first time it is used, when
	// no token file exists there yet. It is never read again afterward.
	RefreshToken string

	// AccountID, BusinessID, and BusinessUUID are the default scope a tool
	// call falls back to when it omits the corresponding field. Stdio
	// only: Validate rejects any of them set alongside --transport http, a
	// shared multi-tenant deployment has no business having a single
	// default scope for every caller (see Validate's doc comment).
	AccountID    string
	BusinessID   int64
	BusinessUUID string

	// BaseURL overrides the FreshBooks API root -- tests and future
	// sandboxes.
	BaseURL string

	// businessIDErr records a FRESHBOOKS_BUSINESS_ID that failed to parse
	// as an int64, so Validate can reject it with a clear message instead
	// of Load silently leaving BusinessID at its zero value.
	businessIDErr error
}

// flagDefs is every cobra flag Load resolves: its name, its
// FRESHBOOKS_MCP_* env twin, its built-in default, and its usage string --
// the single source AddFlags and stringFlag both read, so adding a flag
// never requires editing two places that can drift.
var flagDefs = []struct {
	name, env, def, usage string
}{
	{"transport", "FRESHBOOKS_MCP_TRANSPORT", "stdio", "transport to serve on: stdio or http"},
	{"addr", "FRESHBOOKS_MCP_ADDR", "127.0.0.1:8080", "address to listen on in http mode"},
	{"path", "FRESHBOOKS_MCP_PATH", "/mcp", "URL path to serve the MCP endpoint on in http mode"},
	{"log-level", "FRESHBOOKS_MCP_LOG_LEVEL", "info", "log level: debug, info, warn, or error"},
	{"log-format", "FRESHBOOKS_MCP_LOG_FORMAT", "text", "log format: json or text"},
}

// envForFlag maps a flag name to its FRESHBOOKS_MCP_* env twin, built once
// from flagDefs.
var envForFlag = func() map[string]string {
	m := make(map[string]string, len(flagDefs))
	for _, f := range flagDefs {
		m[f.name] = f.env
	}
	return m
}()

// AddFlags registers serve's flags on cmd with their built-in defaults.
// Cobra parses cmd's flags before RunE runs; call Load from RunE to
// resolve flag > env > default.
func AddFlags(cmd *cobra.Command) {
	for _, f := range flagDefs {
		cmd.Flags().String(f.name, f.def, f.usage)
	}
}

// stringFlag resolves one flag: the value the user passed on the command
// line if they passed one, else its FRESHBOOKS_MCP_* env twin if set, else
// the flag's built-in default.
func stringFlag(cmd *cobra.Command, name string) string {
	f := cmd.Flags().Lookup(name)
	if f == nil {
		return ""
	}
	if cmd.Flags().Changed(name) {
		return f.Value.String()
	}
	// An env var explicitly set to "" is treated as unset, not as the
	// user's chosen empty value: every flag this resolves through has a
	// sensible non-empty default, and a stray empty env var should not
	// silently defeat it.
	if v, ok := os.LookupEnv(envForFlag[name]); ok && v != "" {
		return v
	}
	return f.Value.String()
}

// Load resolves cmd's flags (flag > env > default) and reads the
// token/scope environment. Call it from RunE, after cobra has parsed
// cmd's flags.
func Load(cmd *cobra.Command) *Config {
	cfg := &Config{
		Transport: stringFlag(cmd, "transport"),
		Addr:      stringFlag(cmd, "addr"),
		Path:      stringFlag(cmd, "path"),
		LogLevel:  stringFlag(cmd, "log-level"),
		LogFormat: stringFlag(cmd, "log-format"),

		AccessToken:  os.Getenv("FRESHBOOKS_ACCESS_TOKEN"),
		ClientID:     os.Getenv("FRESHBOOKS_CLIENT_ID"),
		ClientSecret: os.Getenv("FRESHBOOKS_CLIENT_SECRET"),
		TokenFile:    os.Getenv("FRESHBOOKS_TOKEN_FILE"),
		RefreshToken: os.Getenv("FRESHBOOKS_REFRESH_TOKEN"),
		AccountID:    os.Getenv("FRESHBOOKS_ACCOUNT_ID"),
		BusinessUUID: os.Getenv("FRESHBOOKS_BUSINESS_UUID"),
		BaseURL:      os.Getenv("FRESHBOOKS_BASE_URL"),
	}
	if v := os.Getenv("FRESHBOOKS_BUSINESS_ID"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			// Recorded, not discarded: Validate rejects it with a message
			// naming the bad value, rather than the operator discovering
			// later, per call, that "business_id" is silently missing
			// despite the env var being set.
			cfg.businessIDErr = fmt.Errorf("invalid FRESHBOOKS_BUSINESS_ID %q: %w", v, err)
		} else {
			cfg.BusinessID = n
		}
	}
	return cfg
}

// Validate checks that Transport and LogFormat name a value this program
// understands, that LogLevel and Path/Addr parse, that FRESHBOOKS_BUSINESS_ID
// (if set) was a valid integer, and -- for stdio, which owns its own client
// for the life of the process -- that a usable token configuration is
// present. Http mode is not checked for a token: it ignores that
// environment entirely, since its bearer comes from each request. Http mode
// additionally rejects a configured default scope: a shared, multi-tenant
// deployment must not have every caller who omits account_id/business_id
// silently redirected to one operator's account (docs/phases/3/reports/
// security.md finding 2).
func (c *Config) Validate() error {
	switch c.Transport {
	case "stdio", "http":
	default:
		return fmt.Errorf("invalid --transport %q: want stdio or http", c.Transport)
	}
	switch c.LogFormat {
	case "json", "text":
	default:
		return fmt.Errorf("invalid --log-format %q: want json or text", c.LogFormat)
	}
	if _, err := c.logLevel(); err != nil {
		return err
	}
	if !strings.HasPrefix(c.Path, "/") {
		return fmt.Errorf("invalid --path %q: want a path beginning with /", c.Path)
	}
	if _, _, err := net.SplitHostPort(c.Addr); err != nil {
		return fmt.Errorf("invalid --addr %q: %w", c.Addr, err)
	}
	if c.businessIDErr != nil {
		return c.businessIDErr
	}
	if c.Transport == "stdio" && !c.hasStaticToken() && !c.hasRotatingToken() {
		return errors.New("stdio transport needs a token: set FRESHBOOKS_ACCESS_TOKEN, or FRESHBOOKS_CLIENT_ID + FRESHBOOKS_CLIENT_SECRET + FRESHBOOKS_TOKEN_FILE")
	}
	if c.Transport == "http" && c.hasDefaultScope() {
		return errors.New("http transport must not have a default scope: unset FRESHBOOKS_ACCOUNT_ID/FRESHBOOKS_BUSINESS_ID/FRESHBOOKS_BUSINESS_UUID -- each request supplies its own scope, and a shared deployment must not silently redirect a caller who omits it to one operator's account")
	}
	return nil
}

func (c *Config) hasStaticToken() bool { return c.AccessToken != "" }

func (c *Config) hasRotatingToken() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.TokenFile != ""
}

func (c *Config) hasDefaultScope() bool {
	return c.AccountID != "" || c.BusinessID != 0 || c.BusinessUUID != ""
}

func (c *Config) logLevel() (slog.Level, error) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(c.LogLevel)); err != nil {
		return 0, fmt.Errorf("invalid --log-level %q: %w", c.LogLevel, err)
	}
	return l, nil
}

// Logger builds the process logger: stderr always (stdout is the stdio
// transport), at LogLevel, in LogFormat. Call it once (server.New does)
// and reuse the result -- it is not meant to be rebuilt per request.
func (c *Config) Logger() *slog.Logger {
	level, err := c.logLevel()
	if err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if c.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(h)
}

// TokenSource builds the lib TokenSource stdio mode's single, process-lived
// client uses: AccessToken as-is when set, else the rotating
// ClientID/ClientSecret/TokenFile path, seeding TokenFile's store from
// RefreshToken only the first time -- when the file holds no token yet.
func (c *Config) TokenSource(ctx context.Context) (auth.TokenSource, error) {
	if c.hasStaticToken() {
		return auth.StaticTokenSource(c.AccessToken), nil
	}
	if !c.hasRotatingToken() {
		return nil, errors.New("no usable token configuration: set FRESHBOOKS_ACCESS_TOKEN, or FRESHBOOKS_CLIENT_ID + FRESHBOOKS_CLIENT_SECRET + FRESHBOOKS_TOKEN_FILE")
	}
	store := auth.NewFileStore(c.TokenFile)
	if _, err := store.Load(ctx); errors.Is(err, auth.ErrNoToken) {
		if c.RefreshToken == "" {
			return nil, fmt.Errorf("no token stored at %s and FRESHBOOKS_REFRESH_TOKEN is not set to seed one", c.TokenFile)
		}
		if err := store.Save(ctx, &auth.Token{RefreshToken: c.RefreshToken}); err != nil {
			return nil, fmt.Errorf("seeding the token store: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("loading the token store: %w", err)
	}
	return auth.NewTokenSource(auth.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret}, store), nil
}

// LogValue implements slog.LogValuer so logging a Config never renders a
// secret: AccessToken, ClientSecret, and RefreshToken are reported only as
// presence booleans.
func (c *Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("transport", c.Transport),
		slog.String("addr", c.Addr),
		slog.String("path", c.Path),
		slog.String("log_level", c.LogLevel),
		slog.String("log_format", c.LogFormat),
		slog.Bool("has_access_token", c.hasStaticToken()),
		slog.Bool("has_rotating_token", c.hasRotatingToken()),
		slog.String("account_id", c.AccountID),
		slog.Int64("business_id", c.BusinessID),
		slog.String("business_uuid", c.BusinessUUID),
		slog.String("base_url", c.BaseURL),
	)
}

// String renders Config with every secret redacted, so an accidental %v or
// %+v on a Config cannot leak one.
func (c *Config) String() string {
	return fmt.Sprintf(
		"config.Config{Transport:%q, Addr:%q, Path:%q, LogLevel:%q, LogFormat:%q, AccessToken:redacted, ClientSecret:redacted, RefreshToken:redacted, AccountID:%q, BusinessID:%d, BusinessUUID:%q}",
		c.Transport, c.Addr, c.Path, c.LogLevel, c.LogFormat, c.AccountID, c.BusinessID, c.BusinessUUID,
	)
}
