package meta_test

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/elymas/universal-search/internal/meta"
)

// TestModulePath reads go.mod at repo root and asserts module path + go version.
// RED 2: Fails until go.mod exists with the correct module path and go version.
func TestModulePath(t *testing.T) {
	// Walk up from this file's directory to find go.mod
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine source file path")
	}

	dir := filepath.Dir(thisFile)
	var gomod string
	for {
		candidate := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			gomod = candidate
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found in any parent directory")
		}
		dir = parent
	}

	f, err := os.Open(gomod)
	if err != nil {
		t.Fatalf("cannot open go.mod: %v", err)
	}
	defer f.Close()

	var moduleLine, goLine string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") && moduleLine == "" {
			moduleLine = strings.TrimPrefix(line, "module ")
		}
		if strings.HasPrefix(line, "go ") && goLine == "" {
			goLine = strings.TrimPrefix(line, "go ")
		}
	}

	const wantModule = "github.com/elymas/universal-search"
	const wantGo = "1.23"

	if moduleLine != wantModule {
		t.Errorf("module path = %q, want %q", moduleLine, wantModule)
	}
	if !strings.HasPrefix(goLine, wantGo) {
		t.Errorf("go version = %q, want prefix %q", goLine, wantGo)
	}
}

// TestMetaConstants verifies that the meta package exports Version and ModulePath.
func TestMetaConstants(t *testing.T) {
	if meta.Version == "" {
		t.Error("meta.Version must not be empty")
	}
	if meta.ModulePath == "" {
		t.Error("meta.ModulePath must not be empty")
	}
}
