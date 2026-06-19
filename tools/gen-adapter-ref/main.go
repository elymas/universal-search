// gen-adapter-ref: Go tool that uses go/parser AST to extract the 5
// Capabilities fields per adapter from internal/adapters/*/*.go.
// Keyed by SourceID (not per-file) — handles the hn/hackernews mapping
// and social.go's switch+helper-func dispatch for bluesky+x.
//
// Usage:
//
//	gen-adapter-ref -root <project-root>   # emit 10 JSON files
//	gen-adapter-ref -root <project-root> --check  # diff vs committed; exit 1 on drift
//
// Output: docs/content/en/reference/adapters/_generated/{sourceID}.capabilities.json
//
// @MX:NOTE: [AUTO] stdlib-only (go/parser, go/ast, encoding/json, path/filepath).
// @MX:SPEC: SPEC-DOC-002 REQ-ADPDOC-007
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// capabilitiesOutput is the JSON schema emitted per adapter.
type capabilitiesOutput struct {
	SourceID          string    `json:"sourceID"`
	RequiresAuth      bool      `json:"requiresAuth"`
	AuthEnvVars       []string  `json:"authEnvVars"`
	RateLimitPerMin   int       `json:"rateLimitPerMin"`
	DefaultMaxResults int       `json:"defaultMaxResults"`
	SourcePath        string    `json:"sourcePath"`
	SourceLine        int       `json:"sourceLine"`
	ExtractedAt       time.Time `json:"extractedAt"`
}

func main() {
	root := flag.String("root", ".", "project root directory")
	check := flag.Bool("check", false, "diff committed JSON vs freshly extracted; exit 1 on drift")
	outDir := flag.String("out", "", "output directory (default: <root>/docs/content/en/reference/adapters/_generated)")
	flag.Parse()

	code := run(*root, *check, *outDir, os.Stdout, os.Stderr)
	os.Exit(code)
}

// run executes the emit or check operation and returns an exit code.
// Extracted from main() to allow testing without os.Exit.
//
// @MX:ANCHOR: [AUTO] Core emit/check logic; called by main() and integration tests
// @MX:REASON: fan_in >= 3 — main(), TestRunEmit, TestRunCheck all call this
// @MX:SPEC: SPEC-DOC-002 REQ-ADPDOC-007
func run(root string, check bool, outDir string, stdout, stderr io.Writer) int {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "gen-adapter-ref: abs root: %v\n", err)
		return 1
	}

	outputDir := outDir
	if outputDir == "" {
		outputDir = filepath.Join(absRoot, "docs/content/en/reference/adapters/_generated")
	}

	if !check {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			_, _ = fmt.Fprintf(stderr, "gen-adapter-ref: mkdir %s: %v\n", outputDir, err)
			return 1
		}
	}

	adapterSrcBase := filepath.Join(absRoot, "internal/adapters")
	now := time.Now().UTC().Truncate(time.Second)

	var drifted bool
	for sourceID, spec := range registry {
		absFile := filepath.Join(adapterSrcBase, spec.pkgDir, spec.primaryFile)
		relSrcPath := filepath.Join("internal/adapters", spec.pkgDir, spec.primaryFile)

		fields, err := extract(absFile, spec.funcName)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "gen-adapter-ref: extract %s (%s): %v\n", sourceID, absFile, err)
			return 1
		}

		// Verify the extracted SourceID matches the registry key.
		if fields.SourceID != sourceID {
			_, _ = fmt.Fprintf(stderr, "gen-adapter-ref: %s: extracted SourceID %q != registry key %q\n",
				absFile, fields.SourceID, sourceID)
			return 1
		}

		authEnvVars := fields.AuthEnvVars
		if authEnvVars == nil {
			authEnvVars = []string{}
		}

		out := capabilitiesOutput{
			SourceID:          fields.SourceID,
			RequiresAuth:      fields.RequiresAuth,
			AuthEnvVars:       authEnvVars,
			RateLimitPerMin:   fields.RateLimitPerMin,
			DefaultMaxResults: fields.DefaultMaxResults,
			SourcePath:        relSrcPath,
			SourceLine:        fields.SourceLine,
			ExtractedAt:       now,
		}

		outJSON, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "gen-adapter-ref: marshal %s: %v\n", sourceID, err)
			return 1
		}
		outJSON = append(outJSON, '\n')

		outPath := filepath.Join(outputDir, sourceID+".capabilities.json")

		if check {
			// Read committed file and compare (ignoring extractedAt timestamp).
			committed, readErr := os.ReadFile(outPath)
			if readErr != nil {
				_, _ = fmt.Fprintf(stderr, "gen-adapter-ref --check: cannot read committed %s: %v\n", outPath, readErr)
				drifted = true
				continue
			}
			if diffCapabilities(committed, outJSON) {
				_, _ = fmt.Fprintf(stderr, "DRIFT: %s\n  committed extractedAt stripped and compared\n", outPath)
				drifted = true
			}
		} else {
			if err := os.WriteFile(outPath, outJSON, 0o644); err != nil {
				_, _ = fmt.Fprintf(stderr, "gen-adapter-ref: write %s: %v\n", outPath, err)
				return 1
			}
			_, _ = fmt.Fprintf(stdout, "wrote %s\n", outPath)
		}
	}

	if check && drifted {
		_, _ = fmt.Fprintln(stderr, "gen-adapter-ref: drift detected — committed JSON does not match source")
		return 1
	}
	if check && !drifted {
		_, _ = fmt.Fprintln(stdout, "gen-adapter-ref: no drift detected")
	}
	return 0
}

// diffCapabilities compares two JSON bytes ignoring the extractedAt field.
// Returns true if they differ on the 5 capability fields.
func diffCapabilities(committed, fresh []byte) bool {
	var c, f capabilitiesOutput
	if err := json.Unmarshal(committed, &c); err != nil {
		return true
	}
	if err := json.Unmarshal(fresh, &f); err != nil {
		return true
	}
	// Zero out the timestamp before comparison.
	c.ExtractedAt = time.Time{}
	f.ExtractedAt = time.Time{}
	cb, _ := json.Marshal(c)
	fb, _ := json.Marshal(f)
	return string(cb) != string(fb)
}
