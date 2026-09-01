package server

import (
	"context"
	"errors"
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

	// schemas is shared across every *mcp.Server this Server builds --
	// the single one for stdio, and the fresh one per request in stateless
	// HTTP mode -- so a tool's input schema is resolved once for the life
	// of the process, never per request (see internal/tools/schema.go).
	schemas *mcp.SchemaCache
}

// New builds a Server from cfg. version becomes the MCP Implementation
// version reported to clients and the User-Agent this process sends to
// FreshBooks.
func New(cfg *config.Config, version string) *Server {
	return &Server{cfg: cfg, version: version, schemas: mcp.NewSchemaCache()}
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
		freshbooks.WithLogger(s.cfg.Logger()),
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
// empty token.
func bearerToken(r *http.Request) (token string, ok bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, bearerPrefix) {
		return "", false
	}
	token = strings.TrimSpace(strings.TrimPrefix(h, bearerPrefix))
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
func (s *Server) getServer(r *http.Request) *mcp.Server {
	token, _ := bearerToken(r)
	opts := append([]freshbooks.Option{freshbooks.WithTokenSource(auth.StaticTokenSource(token))}, s.clientOptions()...)

	mcpServer := mcp.NewServer(s.implementation(), &mcp.ServerOptions{SchemaCache: s.schemas})
	client, err := freshbooks.NewClient(opts...)
	if err != nil {
		// Every option above is either a constant or already-validated
		// config; this is unreachable in practice. Returning a tool-less
		// server (rather than panicking on a nil client) keeps the
		// process up so the client sees a clean protocol-level failure.
		return mcpServer
	}
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
		Logger:       s.cfg.Logger(),
	})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle(s.cfg.Path, requireBearer(streamable))
	return mux
}

// shutdownGrace bounds how long RunHTTP waits for in-flight requests to
// finish after ctx is canceled.
const shutdownGrace = 10 * time.Second

// RunHTTP serves HTTPHandler on cfg.Addr until ctx is canceled, then shuts
// down gracefully within shutdownGrace.
func (s *Server) RunHTTP(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.HTTPHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()

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
