// Package types_test — package-level invariants for pkg/types/.
package types_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestPkgTypesNoInternalImports walks `go list -deps -json` for pkg/types
// and asserts no transitive dependency is sourced from
// github.com/elymas/universal-search/internal/, github.com/prometheus/, or
// go.opentelemetry.io/. NFR-CORE-003 mandates this SDK boundary.
func TestPkgTypesNoInternalImports(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "list", "-deps", "-json",
		"github.com/elymas/universal-search/pkg/types/...")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	dec := json.NewDecoder(strings.NewReader(string(out)))
	type pkg struct {
		ImportPath string
		Imports    []string
		Standard   bool
	}
	forbidden := []string{
		"github.com/elymas/universal-search/internal/",
		"github.com/prometheus/",
		"go.opentelemetry.io/",
	}
	for {
		var p pkg
		if err := dec.Decode(&p); err != nil {
			break
		}
		// We only care about packages WITHIN pkg/types/...; their imports
		// must not include the forbidden prefixes.
		if !strings.HasPrefix(p.ImportPath, "github.com/elymas/universal-search/pkg/types") {
			continue
		}
		for _, imp := range p.Imports {
			for _, prefix := range forbidden {
				if strings.HasPrefix(imp, prefix) {
					t.Errorf("pkg/types package %q imports %q (forbidden prefix %q)",
						p.ImportPath, imp, prefix)
				}
			}
		}
	}
}
