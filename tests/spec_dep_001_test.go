// Package spec_test validates SPEC-DEP-001 policy enforcement surface.
// Tests cover CI workflow YAML content, renovate.json rules, compose digest pin,
// docs/dependencies.md section presence, and license allowlist script behavior.
package spec_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// projectRoot returns the absolute path to the repository root.
// Tests are run from the tests/ directory or from the repo root.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the current working directory to find go.mod
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
			t.Fatalf("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// ---------------------------------------------------------------------------
// REQ-DEP-003 — Dependency Audit CI
// ---------------------------------------------------------------------------

// TestGovulncheckInvocation verifies that deps-audit.yml invokes govulncheck.
// Accepts either plain `govulncheck ./...` or the JSON-filtered variant
// (`govulncheck -json ./...`) used once stdlib findings became informational
// per the REQ-DEP-003 policy refinement.
func TestGovulncheckInvocation(t *testing.T) {
	root := projectRoot(t)
	content := readFile(t, filepath.Join(root, ".github/workflows/deps-audit.yml"))
	if !strings.Contains(content, "govulncheck ./...") &&
		!strings.Contains(content, "govulncheck -json ./...") {
		t.Error("deps-audit.yml must invoke govulncheck against ./...")
	}
}

// TestPipAuditInvocation verifies that deps-audit.yml invokes pip-audit
// and has a matrix strategy covering all three Python services.
func TestPipAuditInvocation(t *testing.T) {
	root := projectRoot(t)
	content := readFile(t, filepath.Join(root, ".github/workflows/deps-audit.yml"))

	if !strings.Contains(content, "pip-audit") {
		t.Error("deps-audit.yml must contain 'pip-audit'")
	}
	for _, svc := range []string{"researcher", "storm", "embedder"} {
		if !strings.Contains(content, svc) {
			t.Errorf("deps-audit.yml must reference service matrix entry '%s'", svc)
		}
	}
}

// TestPnpmAuditInvocation verifies the pnpm audit job targets the web directory
// and enforces high severity as the failure threshold.
func TestPnpmAuditInvocation(t *testing.T) {
	root := projectRoot(t)
	content := readFile(t, filepath.Join(root, ".github/workflows/deps-audit.yml"))

	if !strings.Contains(content, "pnpm --dir web audit --audit-level=high") {
		t.Error("deps-audit.yml must contain 'pnpm --dir web audit --audit-level=high'")
	}
}

// TestHadolintInvocation verifies that hadolint is invoked with the pinned
// action version and recursive mode enabled.
func TestHadolintInvocation(t *testing.T) {
	root := projectRoot(t)
	content := readFile(t, filepath.Join(root, ".github/workflows/deps-audit.yml"))

	if !strings.Contains(content, "hadolint/hadolint-action@v3.1.0") {
		t.Error("deps-audit.yml must pin hadolint/hadolint-action@v3.1.0")
	}
	if !strings.Contains(content, "recursive: true") {
		t.Error("deps-audit.yml must set 'recursive: true' for hadolint")
	}
}

// ---------------------------------------------------------------------------
// REQ-DEP-006 — Renovate Config
// ---------------------------------------------------------------------------

// TestRenovateConfigValid verifies renovate.json is valid JSON.
func TestRenovateConfigValid(t *testing.T) {
	root := projectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "renovate.json"))
	if err != nil {
		t.Fatalf("renovate.json not found: %v", err)
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("renovate.json is not valid JSON: %v", err)
	}
}

// TestRenovateConfigRules verifies the critical policy fields in renovate.json.
func TestRenovateConfigRules(t *testing.T) {
	root := projectRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "renovate.json"))
	if err != nil {
		t.Fatalf("renovate.json not found: %v", err)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}

	// prConcurrentLimit must be 5
	limit, ok := cfg["prConcurrentLimit"]
	if !ok {
		t.Error("renovate.json missing 'prConcurrentLimit'")
	} else {
		// JSON numbers unmarshal as float64
		if limit.(float64) != 5 {
			t.Errorf("prConcurrentLimit expected 5, got %v", limit)
		}
	}

	// ignorePaths must contain ".moai/**"
	ignorePaths, ok := cfg["ignorePaths"]
	if !ok {
		t.Error("renovate.json missing 'ignorePaths'")
	} else {
		paths := ignorePaths.([]interface{})
		found := false
		for _, p := range paths {
			if p.(string) == ".moai/**" {
				found = true
				break
			}
		}
		if !found {
			t.Error("renovate.json ignorePaths must contain '.moai/**'")
		}
	}

	// docker.enabled must be false
	docker, ok := cfg["docker"]
	if !ok {
		t.Error("renovate.json missing 'docker' section")
	} else {
		dockerMap := docker.(map[string]interface{})
		if dockerMap["enabled"] != false {
			t.Errorf("renovate.json docker.enabled must be false, got %v", dockerMap["enabled"])
		}
	}
}

// ---------------------------------------------------------------------------
// REQ-DEP-005 — SearXNG Digest Pin
// ---------------------------------------------------------------------------

// TestSearXNGDigestPinned verifies the SearXNG image line uses a dated tag or
// sha256 digest and does NOT use :latest.
func TestSearXNGDigestPinned(t *testing.T) {
	root := projectRoot(t)
	content := readFile(t, filepath.Join(root, "deploy/docker-compose.yml"))

	// Must not contain :latest for searxng
	if strings.Contains(content, "searxng/searxng:latest") {
		t.Error("deploy/docker-compose.yml must not reference 'searxng/searxng:latest'")
	}

	// Must match a dated tag or sha256 digest pattern
	re := regexp.MustCompile(`searxng/searxng(@sha256:[a-f0-9]{64}|:[0-9]{4}\.[0-9]{2}\.[0-9]{2}[-a-f0-9]*)`)
	if !re.MatchString(content) {
		t.Error("deploy/docker-compose.yml SearXNG image must be pinned to a dated tag or sha256 digest")
	}
}

// ---------------------------------------------------------------------------
// REQ-DEP-007 — docs/dependencies.md sections
// ---------------------------------------------------------------------------

// TestDepsManifestSections verifies that docs/dependencies.md contains all
// required section headings per REQ-DEP-007.
func TestDepsManifestSections(t *testing.T) {
	root := projectRoot(t)
	content := readFile(t, filepath.Join(root, "docs/dependencies.md"))

	required := []string{
		"Go Dependency Pinning Policy",
		"Future-Dependencies Placeholder",
		"Go Dependencies",
		"Python Dependencies",
		"Web Dependencies",
		"Compose Services",
	}
	for _, section := range required {
		if !strings.Contains(content, section) {
			t.Errorf("docs/dependencies.md missing required section: %q", section)
		}
	}
}

// TestGoFutureDepsListed verifies that the future-dependency table in
// docs/dependencies.md lists the three required packages mapped to their SPEC IDs.
func TestGoFutureDepsListed(t *testing.T) {
	root := projectRoot(t)
	content := readFile(t, filepath.Join(root, "docs/dependencies.md"))

	checks := map[string]string{
		"chi":           "SPEC-IR-001",
		"client_golang": "SPEC-OBS-001",
		"asynq":         "SPEC-LLM-001",
	}
	for pkg, spec := range checks {
		if !strings.Contains(content, pkg) {
			t.Errorf("docs/dependencies.md must mention package %q", pkg)
		}
		if !strings.Contains(content, spec) {
			t.Errorf("docs/dependencies.md must mention SPEC ID %q (for %s)", spec, pkg)
		}
	}
}

// ---------------------------------------------------------------------------
// REQ-DEP-002 — Pre-commit Autoupdate Workflow
// ---------------------------------------------------------------------------

// TestPreCommitAutoupdateWorkflow verifies the autoupdate workflow file
// contains the required steps.
func TestPreCommitAutoupdateWorkflow(t *testing.T) {
	root := projectRoot(t)
	content := readFile(t, filepath.Join(root, ".github/workflows/pre-commit-autoupdate.yml"))

	if !strings.Contains(content, "pre-commit autoupdate") {
		t.Error("pre-commit-autoupdate.yml must contain 'pre-commit autoupdate' step")
	}
	if !strings.Contains(content, "peter-evans/create-pull-request@v7") {
		t.Error("pre-commit-autoupdate.yml must use 'peter-evans/create-pull-request@v7'")
	}
}

// ---------------------------------------------------------------------------
// REQ-DEP-004 — License Allowlist Script
// ---------------------------------------------------------------------------

// TestLicenseAllowlistScriptSyntax verifies scripts/check-license-allowlist.sh
// has the correct shebang, set -euo pipefail, and behaves correctly on fixtures.
func TestLicenseAllowlistScriptSyntax(t *testing.T) {
	root := projectRoot(t)
	scriptPath := filepath.Join(root, "scripts/check-license-allowlist.sh")
	content := readFile(t, scriptPath)

	if !strings.HasPrefix(content, "#!/usr/bin/env bash") {
		t.Error("check-license-allowlist.sh must start with '#!/usr/bin/env bash'")
	}
	if !strings.Contains(content, "set -euo pipefail") {
		t.Error("check-license-allowlist.sh must contain 'set -euo pipefail'")
	}

	// Test with allowed-only fixture
	t.Run("allowed_fixture", func(t *testing.T) {
		dir := t.TempDir()
		licDir := filepath.Join(dir, "licenses")
		if err := os.MkdirAll(licDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		allowedContent := "some-package MIT License\nanother-package Apache-2.0\nthird-pkg BSD-3-Clause\n"
		if err := os.WriteFile(filepath.Join(licDir, "go.txt"), []byte(allowedContent), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(licDir, "python.txt"), []byte(allowedContent), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(licDir, "web.txt"), []byte(allowedContent), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		cmd := exec.Command("bash", scriptPath)
		cmd.Env = append(os.Environ(), "LICENSE_DIR="+licDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("script should exit 0 on allowed-only fixture, got error: %v\noutput: %s", err, out)
		}
	})

	// Test with disallowed fixture — expect exit code 1
	t.Run("disallowed_fixture", func(t *testing.T) {
		dir := t.TempDir()
		licDir := filepath.Join(dir, "licenses")
		if err := os.MkdirAll(licDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		disallowedContent := "bad-package GPL-3.0 License\n"
		if err := os.WriteFile(filepath.Join(licDir, "go.txt"), []byte(disallowedContent), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		// python.txt and web.txt missing — script should handle gracefully
		cmd := exec.Command("bash", scriptPath)
		cmd.Env = append(os.Environ(), "LICENSE_DIR="+licDir)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("script should exit 1 on disallowed license fixture\noutput: %s", out)
		}
	})

	// Test with missing license files — should warn and exit 0
	t.Run("missing_files", func(t *testing.T) {
		dir := t.TempDir()
		licDir := filepath.Join(dir, "licenses")
		if err := os.MkdirAll(licDir, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		// No license files at all
		cmd := exec.Command("bash", scriptPath)
		cmd.Env = append(os.Environ(), "LICENSE_DIR="+licDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("script should exit 0 when license files are missing (warn, skip): %v\noutput: %s", err, out)
		}
	})
}
