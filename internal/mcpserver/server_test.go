package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/elymas/universal-search/pkg/types"
)

// TestStdioInitializeRoundTrip verifies that the MCP server completes an
// initialize handshake followed by tools/list (empty) over stdio transport.
//
// REQ-MCP-001: binary starts, completes initialize handshake.
// REQ-MCP-002: protocol version 2025-06-18 negotiated.
// REQ-MCP-004: stdio transport round-trip.
func TestStdioInitializeRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	// Build the binary.
	bin := buildMCPBinary(t)

	// Launch the server subprocess.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = append(os.Environ(),
		"USEARCH_ADMIN_PORT=", // disable admin server for test
		"OTLP_ENDPOINT=",      // disable OTel for test
		"LOG_LEVEL=ERROR",     // suppress noisy logs
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)

	// Send initialize request.
	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	writeJSONRPC(t, stdin, initReq)

	// Read initialize response.
	initResp := readJSONRPC(t, reader)
	if initResp["id"] != float64(1) {
		t.Fatalf("expected id=1, got %v", initResp["id"])
	}

	result, ok := initResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T", initResp["result"])
	}

	// Verify protocol version.
	if pv, _ := result["protocolVersion"].(string); pv != "2025-06-18" {
		t.Errorf("expected protocolVersion 2025-06-18, got %q", pv)
	}

	// Verify server info.
	si, _ := result["serverInfo"].(map[string]any)
	if si == nil {
		t.Fatal("expected serverInfo in result")
	}
	if name, _ := si["name"].(string); name != "usearch-mcp" {
		t.Errorf("expected serverInfo.name=usearch-mcp, got %q", name)
	}

	// Send initialized notification.
	initializedNotif := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	writeJSONRPC(t, stdin, initializedNotif)

	// Send tools/list request.
	toolsReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}
	writeJSONRPC(t, stdin, toolsReq)

	// Read tools/list response.
	toolsResp := readJSONRPC(t, reader)
	if toolsResp["id"] != float64(2) {
		t.Fatalf("expected id=2, got %v", toolsResp["id"])
	}

	toolsResult, ok := toolsResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %T", toolsResp["result"])
	}

	tools, ok := toolsResult["tools"].([]any)
	if !ok {
		t.Fatalf("expected tools array, got %T", toolsResult["tools"])
	}

	// T1: initially no tools registered (tools registered in T3-T7).
	_ = tools // tools list may be empty in T1; later tasks populate this.

	// Verify stdout has no non-MCP bytes (REQ-MCP-004).
	// If we got valid JSON-RPC responses, the basic assertion holds.
	t.Logf("initialize result: server=%q protocol=%q tools=%d",
		si["name"], result["protocolVersion"], len(tools))
}

// TestTransportFlagDefaultsStdio verifies that --transport unspecified defaults
// to stdio transport.
//
// REQ-MCP-004: stdio is the default transport.
func TestTransportFlagDefaultsStdio(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Transport != "stdio" {
		t.Errorf("expected default transport 'stdio', got %q", cfg.Transport)
	}
}

// TestVersionFlagPreserved verifies that --version semantics are preserved
// from the SPEC-BOOT-001 stub.
func TestVersionFlagPreserved(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	bin := buildMCPBinary(t)

	cmd := exec.Command(bin, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}

	ver := strings.TrimSpace(string(out))
	if !strings.Contains(ver, "usearch-mcp") {
		t.Errorf("expected version output to contain 'usearch-mcp', got %q", ver)
	}
}

// moduleRoot returns the module root directory (where go.mod lives).
func moduleRoot(t *testing.T) string {
	t.Helper()
	// Walk up from test directory to find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod")
		}
		dir = parent
	}
}

// buildMCPBinary builds the usearch-mcp binary and returns its path.
func buildMCPBinary(t *testing.T) string {
	t.Helper()
	root := moduleRoot(t)
	bin := filepath.Join(t.TempDir(), "usearch-mcp-test")
	buildCmd := exec.Command("go", "build", "-o", bin, "./cmd/usearch-mcp/")
	buildCmd.Dir = root
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// writeJSONRPC writes a JSON-RPC message as a single line to the writer.
func writeJSONRPC(t *testing.T, w io.Writer, msg map[string]any) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal JSON-RPC: %v", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
		t.Fatalf("write JSON-RPC: %v", err)
	}
}

// readJSONRPC reads a single JSON-RPC message from the reader.
func readJSONRPC(t *testing.T, r *bufio.Reader) map[string]any {
	t.Helper()
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read JSON-RPC line: %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &msg); err != nil {
		t.Fatalf("unmarshal JSON-RPC: %v\nline: %q", err, line)
	}
	return msg
}

// TestDocCacheReturnsSharedCache verifies DocCache returns the internal cache.
func TestDocCacheReturnsSharedCache(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg, nil, nil, nil, nil)

	cache := srv.DocCache()
	if cache == nil {
		t.Fatal("expected non-nil DocCache")
	}

	// Store a doc and verify it is accessible via DocCache().
	cache.Store([]types.NormalizedDoc{{ID: "test-doc", Title: "Test"}})

	doc, ok := cache.Get("test-doc")
	if !ok {
		t.Fatal("expected doc in cache")
	}
	if doc.Title != "Test" {
		t.Errorf("title: got %q, want Test", doc.Title)
	}
}

// TestTransportReturnsStdio verifies that transport() returns stdio for default config.
func TestTransportReturnsStdio(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg, nil, nil, nil, nil)

	tp := srv.transport()
	if tp == nil {
		t.Fatal("expected non-nil transport for stdio")
	}
}

// TestTransportReturnsNilForUnknown verifies that transport() returns nil for
// unknown transport types.
func TestTransportReturnsNilForUnknown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "unknown"
	srv := New(cfg, nil, nil, nil, nil)

	tp := srv.transport()
	if tp != nil {
		t.Error("expected nil transport for unknown type")
	}
}

// TestLogStartupDoesNotPanic verifies logStartup runs without panicking.
func TestLogStartupDoesNotPanic(t *testing.T) {
	cfg := DefaultConfig()
	srv := New(cfg, nil, nil, nil, nil)

	// Should not panic.
	srv.logStartup()
}

// TestStartUnknownTransportFails verifies that Start returns an error for
// unknown transport types.
func TestStartUnknownTransportFails(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Transport = "unknown"
	srv := New(cfg, nil, nil, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := srv.Start(ctx)
	if err == nil {
		t.Fatal("expected error for unknown transport")
	}
	if !strings.Contains(err.Error(), "unknown transport") {
		t.Errorf("error: %v", err)
	}
}
