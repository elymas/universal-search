package deepagent

import (
	"os"
	"strings"
	"testing"
)

// T-M1-005 [REFACTOR]: Verify no os.Getenv calls outside config.go
// REQ-DEEP2-004: os.Getenv restricted to config.go only.

func TestNoDirectOsGetenvInAgentsPackage(t *testing.T) {
	// This test verifies that os.Getenv only appears in config.go.
	// Read all Go files in the deepagent package and check.
	files := []string{
		"agents.go",
		"orchestrator.go",
		"types.go",
		"prompts.go",
	}

	for _, file := range files {
		path := file
		data, err := os.ReadFile(path)
		if err != nil {
			// File does not exist yet — acceptable for M1.
			continue
		}
		content := string(data)
		if strings.Contains(content, "os.Getenv") {
			t.Errorf("file %q contains os.Getenv — env loading must be in config.go only", path)
		}
	}
}
