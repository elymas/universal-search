package mcpserver

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/elymas/universal-search/internal/adapters"
	"github.com/elymas/universal-search/internal/deepagent/costguard"
	"github.com/elymas/universal-search/internal/mcpserver/tools"
	"github.com/elymas/universal-search/internal/obs"
)

// Server is the MCP server wrapping the SDK server with lifecycle management.
//
// @MX:ANCHOR: [AUTO] MCP server lifecycle entry point; callers: cmd/usearch-mcp, tests
// @MX:REASON: fan_in >= 2; sole server lifecycle entry; signature stability matters
// for SDK upgrade and future transport plugins.
// @MX:SPEC: SPEC-MCP-001
type Server struct {
	cfg       Config
	obs       *obs.Obs
	sdk       *mcp.Server
	reg       *adapters.Registry
	cache     *tools.DocCache
	capCheck  tools.CapChecker
	pipeline  tools.PipelineFn
}

// New creates a new MCP server with the given configuration and observability bundle.
// If reg is nil, no adapter-dependent tools are registered.
// capCheck and pipeline are optional; when nil, deep_research is not registered.
func New(cfg Config, o *obs.Obs, reg *adapters.Registry, capCheck *costguard.CapChecker, pipeline tools.PipelineFn) *Server {
	s := &Server{
		cfg:      cfg,
		obs:      o,
		reg:      reg,
		cache:    tools.NewDocCache(),
		capCheck: capCheck,
		pipeline: pipeline,
	}

	logger := s.logger()
	s.sdk = mcp.NewServer(
		&mcp.Implementation{
			Name:    "usearch-mcp",
			Version: "0.1.0",
		},
		&mcp.ServerOptions{
			Logger: logger,
		},
	)

	// Register tools based on available dependencies.
	if reg != nil {
		s.registerTools()
	}

	return s
}

// registerTools registers all MCP tools.
func (s *Server) registerTools() {
	mcp.AddTool(s.sdk, tools.ListSourcesTool(), tools.ListSourcesHandler(s.reg))
	mcp.AddTool(s.sdk, tools.GetCitationTool(), tools.GetCitationHandler(s.cache))
	mcp.AddTool(s.sdk, tools.SearchTool(), tools.SearchHandler(s.reg, s.cache))

	// Register deep_research only when cap checker and pipeline are provided.
	if s.capCheck != nil && s.pipeline != nil {
		auditFn := func(line string) error {
			s.logger().Info("audit", "line", line)
			return nil
		}
		notifyFn := func(method string, data map[string]any) {
			s.logger().Info("progress", "method", method, "data", data)
		}
		handler := tools.DeepResearchHandler(s.capCheck, s.pipeline, notifyFn, auditFn)
		mcp.AddTool(s.sdk, tools.DeepResearchTool(), handler)
	}
}

// DocCache returns the shared document cache for storing fanout results.
func (s *Server) DocCache() *tools.DocCache {
	return s.cache
}

// Start runs the MCP server on the configured transport until ctx is cancelled
// or a termination signal is received.
//
// REQ-MCP-003: Graceful shutdown with grace period for in-flight requests.
// @MX:SPEC: SPEC-MCP-001
func (s *Server) Start(ctx context.Context) error {
	// Set up signal handling.
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	transport := s.transport()
	if transport == nil {
		return fmt.Errorf("mcpserver: unknown transport %q", s.cfg.Transport)
	}

	s.logStartup()
	if err := s.sdk.Run(ctx, transport); err != nil && ctx.Err() == nil {
		return fmt.Errorf("mcpserver: server run: %w", err)
	}

	// Perform graceful shutdown.
	s.gracefulShutdown(ctx)
	return nil
}

// transport returns the MCP transport based on configuration.
func (s *Server) transport() mcp.Transport {
	switch s.cfg.Transport {
	case "stdio", "":
		return &mcp.StdioTransport{}
	default:
		// HTTP transport implemented in T8.
		return nil
	}
}

// logger returns an slog.Logger for the MCP SDK, preferring obs.Logger or falling
// back to a default.
func (s *Server) logger() *slog.Logger {
	if s.obs != nil && s.obs.Logger != nil {
		return s.obs.Logger
	}
	return slog.Default()
}

// logStartup emits a structured startup log.
func (s *Server) logStartup() {
	logger := s.logger()
	logger.Info("usearch-mcp starting",
		"transport", s.cfg.Transport,
	)
}

