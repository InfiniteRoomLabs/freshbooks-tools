package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
	"github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/config"
	"github.com/InfiniteRoomLabs/freshbooks-tools/mcp/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server builds and runs freshbooks-mcp over either transport.
type Server struct {
	cfg     *config.Config
	version string
	logger  *slog.Logger

	// schemas is shared across every *mcp.Server this Server builds --
	// the single one for stdio, and the fresh one per request in stateless
	// HTTP mode -- so a tool's input schema is resolved once for the life
	// of the process, never per request (see internal/tools/schema.go).
	schemas *mcp.SchemaCache
}

// New builds a Server from cfg. version becomes the MCP Implementation
// version reported to clients and the User-Agent this process sends to
// FreshBooks. The logger is built once here and reused for the life of
// the process -- not rebuilt per request, which cfg.Logger() alone would
// do if called from a hot path.
func New(cfg *config.Config, version string) *Server {
	return &Server{cfg: cfg, version: version, logger: cfg.Logger(), schemas: mcp.NewSchemaCache()}
}

func (s *Server) implementation() *mcp.Implementation {
	return &mcp.Implementation{Name: "freshbooks-mcp", Version: s.version}
}

func (s *Server) userAgent() string {
	return "freshbooks-mcp/" + s.version
}

func (s *Server) defaultScope() tools.Scope {
	return tools.Scope{
		AccountID:    freshbooks.AccountID(s.cfg.AccountID),
		BusinessID:   freshbooks.BusinessID(s.cfg.BusinessID),
		BusinessUUID: freshbooks.BusinessUUID(s.cfg.BusinessUUID),
	}
}

// clientOptions builds the freshbooks.Option slice every client this
// server constructs shares, minus the token source (which differs between
// the single stdio client and each per-request HTTP client).
func (s *Server) clientOptions() []freshbooks.Option {
	opts := []freshbooks.Option{
		freshbooks.WithUserAgent(s.userAgent()),
		freshbooks.WithLogger(s.logger),
	}
	if s.cfg.BaseURL != "" {
		opts = append(opts, freshbooks.WithBaseURL(s.cfg.BaseURL))
	}
	return opts
}

// RunStdio builds one lib client from the process's configured token
// source, registers every tool onto one *mcp.Server, and serves stdio
// until ctx is canceled or the transport errors.
func (s *Server) RunStdio(ctx context.Context) error {
	ts, err := s.cfg.TokenSource(ctx)
	if err != nil {
		return err
	}
	opts := append([]freshbooks.Option{freshbooks.WithTokenSource(ts)}, s.clientOptions()...)
	client, err := freshbooks.NewClient(opts...)
	if err != nil {
		return err
	}

	mcpServer := mcp.NewServer(s.implementation(), &mcp.ServerOptions{SchemaCache: s.schemas})
	tools.Register(mcpServer, client, s.defaultScope())
	return mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// bearerPrefix is the Authorization scheme every HTTP request must use.
const bearerPrefix = "Bearer "

// bearerToken extracts the token from r's Authorization header. ok is
// false when the header is missing, uses a different scheme, or carries an
// empty token. The scheme match is case-insensitive (RFC 7235 section
// 2.1: "auth-scheme" is case-insensitive), so "bearer tok" and "BEARER
// tok" are accepted exactly like "Bearer tok" -- a client is not required
// to send the exact casing this server happens to document.
func bearerToken(r *http.Request) (token string, ok bool) {
	h := r.Header.Get("Authorization")
	if len(h) < len(bearerPrefix) || !strings.EqualFold(h[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}
	token = strings.TrimSpace(h[len(bearerPrefix):])
	return token, token != ""
}

// requireBearer rejects a request with 401 and a WWW-Authenticate header
// before any JSON-RPC parsing when it carries no usable bearer token.
func requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := bearerToken(r); !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="freshbooks-mcp"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getServer builds a fresh, per-request *mcp.Server bound to a lib client
// authenticated with the request's bearer token. It shares s.schemas with
// every other server this process builds, and is only ever invoked after
// requireBearer has already confirmed the header is present, so the
// client's token source is never empty in practice.
//
// It returns nil when freshbooks.NewClient fails -- every option passed to
// it today is either a constant or already-validated config, so this is
// unreachable in practice, but nil (rather than a server with zero tools
// registered) makes go-sdk answer a clean "400 no server available"
// instead of every subsequent call failing as "unknown tool", the most
// confusing failure mode available to a client (docs/phases/3/reports/
// code-review.md finding 10, security.md finding 7).
func (s *Server) getServer(r *http.Request) *mcp.Server {
	token, _ := bearerToken(r)
	opts := append([]freshbooks.Option{freshbooks.WithTokenSource(auth.StaticTokenSource(token))}, s.clientOptions()...)

	client, err := freshbooks.NewClient(opts...)
	if err != nil {
		s.logger.Error("building the freshbooks client for a request", "error", err)
		return nil
	}
	mcpServer := mcp.NewServer(s.implementation(), &mcp.ServerOptions{SchemaCache: s.schemas})
	tools.Register(mcpServer, client, s.defaultScope())
	return mcpServer
}

// HTTPHandler builds the stateless streamable-HTTP handler: GET /healthz
// returns 200 unauthenticated; every request under cfg.Path requires a
// bearer token (401 + WWW-Authenticate otherwise) and is served by a
// per-request server built from that token -- no session, no cache keyed
// by client or token, nothing written to disk.
func (s *Server) HTTPHandler() http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(s.getServer, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		Logger:       s.logger,
		// CrossOriginProtection rejects a browser cross-origin request by
		// default (its zero value trusts no origin); go-sdk v1.7.0 only
		// enables this when the MCPGODEBUG compatibility parameter
		// enableoriginverification=1 is set, which this process does not
		// set. Setting it explicitly here does not depend on that
		// parameter now or if the SDK's default ever changes (see
		// docs/phases/3/reports/security.md finding 3). The field itself
		// is deprecated in go-sdk v1.7.0 in favor of wrapping the handler
		// in http.CrossOriginProtection.Handler middleware; this uses the
		// field anyway because it is what the review lane's fix asked
		// for, it is still fully functional, and it keeps the protection
		// declared alongside every other StreamableHTTPOptions setting
		// rather than as a separate wrapping step easy to forget when
		// this handler is next touched.
		CrossOriginProtection: &http.CrossOriginProtection{}, //nolint:staticcheck // SA1019: deliberate, see comment above
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle(s.cfg.Path, requireBearer(streamable))
	return mux
}

// shutdownGrace bounds how long Serve waits for in-flight requests to
// finish after ctx is canceled.
const shutdownGrace = 10 * time.Second

// Serve runs HTTPHandler on l until ctx is canceled, then shuts down
// gracefully within shutdownGrace. RunHTTP is a thin wrapper that binds
// cfg.Addr and calls this; tests call Serve directly with a listener
// they bind themselves (an ephemeral "127.0.0.1:0" port), so the bind is
// synchronous and there is nothing to poll or sleep for before the server
// is known to be listening.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	httpServer := &http.Server{
		Handler:           s.HTTPHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.Serve(l) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// RunHTTP binds cfg.Addr and serves HTTPHandler on it until ctx is
// canceled, then shuts down gracefully.
func (s *Server) RunHTTP(ctx context.Context) error {
	l, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, l)
}
