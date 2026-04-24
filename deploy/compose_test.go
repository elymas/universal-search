//go:build integration

package deploy_test

import (
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestComposeConfigValid is RED 4: verifies docker compose config is valid
// and every service has a healthcheck.
// Requires docker compose to be available; skips otherwise.
func TestComposeConfigValid(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available in this environment")
	}

	out, err := exec.Command("docker", "compose", "-f", "docker-compose.yml", "config").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("docker compose config failed: %s", exitErr.Stderr)
		}
		t.Fatalf("docker compose config error: %v", err)
	}

	var cfg struct {
		Services map[string]struct {
			Healthcheck *struct {
				Test interface{} `yaml:"test"`
			} `yaml:"healthcheck"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("cannot parse compose config output: %v", err)
	}

	for svc, def := range cfg.Services {
		if def.Healthcheck == nil {
			t.Errorf("service %q is missing a healthcheck stanza", svc)
		}
	}

	// Verify no hardcoded credentials in rendered output
	rendered := string(out)
	suspectPatterns := []string{"password: dev", "secret: dev", "password: abc"}
	for _, pat := range suspectPatterns {
		if strings.Contains(rendered, pat) {
			t.Errorf("rendered compose config appears to contain hardcoded credential: %q", pat)
		}
	}
}
