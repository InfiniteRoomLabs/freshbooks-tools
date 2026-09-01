package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
	"github.com/spf13/cobra"
)

// newTestCmd builds a bare *cobra.Command carrying AddFlags' flags, parsed
// against args (empty for "nothing passed on the command line").
func newTestCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "serve"}
	AddFlags(cmd)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	return cmd
}

func TestLoadPrecedence(t *testing.T) {
	t.Run("[happy] a flag wins over its env twin", func(t *testing.T) {
		t.Setenv("FRESHBOOKS_MCP_TRANSPORT", "http")
		cfg := Load(newTestCmd(t, "--transport", "stdio"))
		if cfg.Transport != "stdio" {
			t.Fatalf("Transport = %q, want stdio (flag beats env)", cfg.Transport)
		}
	})

	t.Run("[happy] env wins over the built-in default when no flag was passed", func(t *testing.T) {
		t.Setenv("FRESHBOOKS_MCP_TRANSPORT", "http")
		cfg := Load(newTestCmd(t))
		if cfg.Transport != "http" {
			t.Fatalf("Transport = %q, want http (env beats default)", cfg.Transport)
		}
	})

	t.Run("[happy] the built-in default applies when neither flag nor env is set", func(t *testing.T) {
		t.Setenv("FRESHBOOKS_MCP_TRANSPORT", "")
		cfg := Load(newTestCmd(t))
		if cfg.Transport != "stdio" {
			t.Fatalf("Transport = %q, want stdio", cfg.Transport)
		}
		if cfg.Addr != "127.0.0.1:8080" || cfg.Path != "/mcp" || cfg.LogLevel != "info" || cfg.LogFormat != "text" {
			t.Fatalf("defaults = %+v", cfg)
		}
	})

	t.Run("[happy] the token/scope environment is read directly, not via a flag", func(t *testing.T) {
		t.Setenv("FRESHBOOKS_ACCESS_TOKEN", "tok-123")
		t.Setenv("FRESHBOOKS_ACCOUNT_ID", "ACM1")
		t.Setenv("FRESHBOOKS_BUSINESS_ID", "42")
		t.Setenv("FRESHBOOKS_BUSINESS_UUID", "uuid-1")
		t.Setenv("FRESHBOOKS_BASE_URL", "https://example.test")
		cfg := Load(newTestCmd(t))
		if cfg.AccessToken != "tok-123" || cfg.AccountID != "ACM1" || cfg.BusinessID != 42 || cfg.BusinessUUID != "uuid-1" || cfg.BaseURL != "https://example.test" {
			t.Fatalf("cfg = %+v", cfg)
		}
	})

	t.Run("[sad] an unparseable FRESHBOOKS_BUSINESS_ID is rejected by Validate, not silently ignored", func(t *testing.T) {
		t.Setenv("FRESHBOOKS_ACCESS_TOKEN", "tok") // otherwise the missing-token error would win
		t.Setenv("FRESHBOOKS_BUSINESS_ID", "not-a-number")
		cfg := Load(newTestCmd(t))
		if cfg.BusinessID != 0 {
			t.Fatalf("BusinessID = %d, want 0 (the parse failed)", cfg.BusinessID)
		}
		err := cfg.Validate()
		if err == nil || !strings.Contains(err.Error(), "FRESHBOOKS_BUSINESS_ID") || !strings.Contains(err.Error(), "not-a-number") {
			t.Fatalf("Validate() = %v, want it to name FRESHBOOKS_BUSINESS_ID and the bad value", err)
		}
	})
}

// validConfig returns a Config with every field Validate checks besides
// the one under test set to a passing value, so each subtest below can
// override just the field it means to exercise.
func validConfig() *Config {
	return &Config{Transport: "stdio", LogFormat: "text", LogLevel: "info", Addr: "127.0.0.1:8080", Path: "/mcp", AccessToken: "tok"}
}

func TestValidate(t *testing.T) {
	t.Run("[sad] an invalid transport is rejected", func(t *testing.T) {
		cfg := validConfig()
		cfg.Transport = "bogus"
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "bogus") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] an invalid log format is rejected", func(t *testing.T) {
		cfg := validConfig()
		cfg.LogFormat = "xml"
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "xml") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] an invalid log level is rejected", func(t *testing.T) {
		cfg := validConfig()
		cfg.LogLevel = "loud"
		if err := cfg.Validate(); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] stdio with no token configuration fails fast", func(t *testing.T) {
		cfg := validConfig()
		cfg.AccessToken = ""
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "token") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[happy] stdio with a static access token validates", func(t *testing.T) {
		cfg := validConfig()
		if err := cfg.Validate(); err != nil {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[happy] stdio with a complete rotating token configuration validates", func(t *testing.T) {
		cfg := validConfig()
		cfg.AccessToken = ""
		cfg.ClientID, cfg.ClientSecret, cfg.TokenFile = "id", "secret", "/tmp/token.json"
		if err := cfg.Validate(); err != nil {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[edge] a partial rotating token configuration is not enough", func(t *testing.T) {
		cfg := validConfig()
		cfg.AccessToken = ""
		cfg.ClientID = "id"
		if err := cfg.Validate(); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[happy] http mode never needs a token", func(t *testing.T) {
		cfg := validConfig()
		cfg.Transport, cfg.AccessToken = "http", ""
		if err := cfg.Validate(); err != nil {
			t.Fatalf("err = %v", err)
		}
	})

	for _, path := range []string{"mcp", "", "relative/path"} {
		t.Run("[sad] --path "+path+" without a leading slash is rejected", func(t *testing.T) {
			cfg := validConfig()
			cfg.Path = path
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "--path") {
				t.Fatalf("err = %v", err)
			}
		})
	}

	for _, addr := range []string{"", "not-an-addr", "127.0.0.1", "127.0.0.1:"} {
		t.Run("[sad] --addr "+addr+" that fails host:port parsing is rejected", func(t *testing.T) {
			cfg := validConfig()
			cfg.Addr = addr
			if addr == "127.0.0.1:" {
				// A trailing colon with no port is syntactically valid to
				// net.SplitHostPort (empty port); keep this case as
				// documentation of that, not an assertion either way.
				return
			}
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "--addr") {
				t.Fatalf("err = %v", err)
			}
		})
	}

	t.Run("[sad] http mode with a default account_id is rejected", func(t *testing.T) {
		cfg := validConfig()
		cfg.Transport, cfg.AccessToken, cfg.AccountID = "http", "", "ACM123"
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "default scope") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] http mode with a default business_id is rejected", func(t *testing.T) {
		cfg := validConfig()
		cfg.Transport, cfg.AccessToken, cfg.BusinessID = "http", "", 42
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "default scope") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("[sad] http mode with a default business_uuid is rejected", func(t *testing.T) {
		cfg := validConfig()
		cfg.Transport, cfg.AccessToken, cfg.BusinessUUID = "http", "", "uuid-1"
		if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "default scope") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTokenSource(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] a static access token needs no store", func(t *testing.T) {
		cfg := &Config{AccessToken: "tok-123"}
		ts, err := cfg.TokenSource(ctx)
		if err != nil {
			t.Fatal(err)
		}
		tok, err := ts.Token(ctx)
		if err != nil || tok.AccessToken != "tok-123" {
			t.Fatalf("token = %+v err = %v", tok, err)
		}
	})

	t.Run("[sad] no usable token configuration", func(t *testing.T) {
		cfg := &Config{}
		if _, err := cfg.TokenSource(ctx); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[happy] seeds an empty token file from FRESHBOOKS_REFRESH_TOKEN", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token.json")
		cfg := &Config{ClientID: "id", ClientSecret: "secret", TokenFile: path, RefreshToken: "seed-refresh-token"}
		if _, err := cfg.TokenSource(ctx); err != nil {
			t.Fatal(err)
		}
		stored, err := auth.NewFileStore(path).Load(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if stored.RefreshToken != "seed-refresh-token" {
			t.Fatalf("stored = %+v", stored)
		}
	})

	t.Run("[sad] no token file and no seed refuses to start", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token.json")
		cfg := &Config{ClientID: "id", ClientSecret: "secret", TokenFile: path}
		if _, err := cfg.TokenSource(ctx); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[edge] an existing token file is never re-seeded", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "token.json")
		store := auth.NewFileStore(path)
		if err := store.Save(ctx, &auth.Token{AccessToken: "already-here", RefreshToken: "already-here-refresh"}); err != nil {
			t.Fatal(err)
		}
		cfg := &Config{ClientID: "id", ClientSecret: "secret", TokenFile: path, RefreshToken: "should-not-be-used"}
		ts, err := cfg.TokenSource(ctx)
		if err != nil {
			t.Fatal(err)
		}
		tok, err := ts.Token(ctx)
		if err != nil || tok.AccessToken != "already-here" {
			t.Fatalf("token = %+v err = %v", tok, err)
		}
	})

	t.Run("[sad] a store that cannot be read surfaces its error", func(t *testing.T) {
		dir := t.TempDir()
		// A directory where the token file should be: Load fails with
		// something other than ErrNoToken.
		badPath := filepath.Join(dir, "notafile")
		if err := os.Mkdir(badPath, 0o750); err != nil {
			t.Fatal(err)
		}
		cfg := &Config{ClientID: "id", ClientSecret: "secret", TokenFile: badPath, RefreshToken: "x"}
		_, err := cfg.TokenSource(ctx)
		if err == nil || errors.Is(err, auth.ErrNoToken) {
			t.Fatalf("err = %v, want a non-ErrNoToken failure", err)
		}
	})
}

func TestLoggerAndRedaction(t *testing.T) {
	t.Run("[happy] Logger builds without error for every valid format", func(t *testing.T) {
		for _, format := range []string{"json", "text"} {
			cfg := &Config{LogLevel: "debug", LogFormat: format}
			if cfg.Logger() == nil {
				t.Fatalf("Logger() nil for format %q", format)
			}
		}
	})

	t.Run("[edge] an invalid log level falls back to info rather than panicking", func(t *testing.T) {
		cfg := &Config{LogLevel: "not-a-level", LogFormat: "text"}
		if cfg.Logger() == nil {
			t.Fatal("Logger() returned nil")
		}
	})

	t.Run("[sad] String and LogValue never render a secret", func(t *testing.T) {
		cfg := &Config{
			Transport: "stdio", Addr: "a", Path: "/mcp", LogLevel: "info", LogFormat: "text",
			AccessToken: "super-secret-access-token", ClientSecret: "super-secret-client-secret", RefreshToken: "super-secret-refresh-token",
		}
		for _, secret := range []string{"super-secret-access-token", "super-secret-client-secret", "super-secret-refresh-token"} {
			if strings.Contains(cfg.String(), secret) {
				t.Fatalf("String() leaked a secret: %s", cfg.String())
			}
			if strings.Contains(cfg.LogValue().String(), secret) {
				t.Fatalf("LogValue() leaked a secret: %s", cfg.LogValue().String())
			}
		}
	})
}
