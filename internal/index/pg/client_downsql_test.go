// SPEC-DEPLOY-001 D2 — characterization test for the *.down.sql exclusion in
// the migration file selector. EnsureSchema (and the Helm pre-install/pre-upgrade
// migrate Job that reuses it) must never exec down-migrations on forward apply,
// because they sort lexicographically before their .up.sql counterparts and would
// drop populated tables on re-run/upgrade.
package pg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectMigrationFiles_ExcludesDownSQL(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Mirror the real deploy/postgres/migrations layout: duplicate version
	// prefixes, mixed bare-.sql / .up.sql / .down.sql suffixes.
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	for _, n := range []string{
		"0001_create_docs.sql",
		"0002_cost_ledger.sql",
		"0002_deep_runs.down.sql", // MUST be excluded — DROP TABLE on forward apply
		"0002_deep_runs.up.sql",
		"0003_audit_events.sql",
		"0003_casbin_rules.up.sql",
		"0007_answer_cache.up.sql",
		"notes.txt", // non-.sql — excluded
	} {
		write(n)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := selectMigrationFiles(dir)
	if err != nil {
		t.Fatalf("selectMigrationFiles: %v", err)
	}

	want := []string{
		"0001_create_docs.sql",
		"0002_cost_ledger.sql",
		"0002_deep_runs.up.sql",
		"0003_audit_events.sql",
		"0003_casbin_rules.up.sql",
		"0007_answer_cache.up.sql",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d files %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("file[%d] = %q, want %q (order must be lexicographic)", i, got[i], want[i])
		}
	}

	// Explicit guard: no .down.sql may ever appear in the forward-apply set.
	for _, f := range got {
		if filepath.Ext(f) == ".sql" && len(f) >= len(".down.sql") &&
			f[len(f)-len(".down.sql"):] == ".down.sql" {
			t.Errorf("down-migration %q leaked into forward-apply set", f)
		}
	}
}

// TestSelectMigrationFiles_RealMigrationsDir asserts the behavior against the
// actual repository migrations directory: the real 0002_deep_runs.down.sql is
// present on disk but must never be selected for forward apply.
func TestSelectMigrationFiles_RealMigrationsDir(t *testing.T) {
	t.Parallel()

	// Resolve repo-root migrations dir relative to this test file.
	dir := filepath.Join("..", "..", "..", "deploy", "postgres", "migrations")
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("real migrations dir not present: %v", err)
	}

	got, err := selectMigrationFiles(dir)
	if err != nil {
		t.Fatalf("selectMigrationFiles: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one forward migration")
	}
	for _, f := range got {
		if f == "0002_deep_runs.down.sql" {
			t.Fatalf("0002_deep_runs.down.sql must NOT be applied on forward apply")
		}
	}
}
