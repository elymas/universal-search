// Package koreanews — goroutine leak detection via goleak.VerifyTestMain.
// SPEC-ADP-009 §11.6: all tests in the package must be goroutine-leak-clean.
package koreanews_test

import (
	"os"
	"testing"

	"go.uber.org/goleak"
)

// TestMain installs goleak to catch goroutine leaks across all tests in the package.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
	os.Exit(m.Run())
}
