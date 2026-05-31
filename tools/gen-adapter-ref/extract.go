// Package main — extract.go: AST walker for Capabilities() struct literals.
// Driven by a SourceID-keyed registry (not a naive per-file glob).
// Handles standard adapters + the special cases:
//   - hn/ package dir → SourceID "hackernews"
//   - social/social.go switch-dispatch → blueskyCapabilities()/xCapabilities() helpers
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// capabilitiesFields holds the 5 fields we extract from each Capabilities struct literal.
type capabilitiesFields struct {
	SourceID          string
	RequiresAuth      bool
	AuthEnvVars       []string
	RateLimitPerMin   int
	DefaultMaxResults int
	SourcePath        string
	SourceLine        int
}

// adapterSpec describes where to find a specific adapter's Capabilities.
type adapterSpec struct {
	// pkgDir is the sub-directory under internal/adapters/
	pkgDir string
	// primaryFile is the Go file containing the Capabilities() method or helper func
	primaryFile string
	// funcName is the function/method that returns the Capabilities literal.
	// For standard adapters it is "" (we look for Capabilities() method receiver).
	// For social helpers we use "blueskyCapabilities" or "xCapabilities".
	funcName string
}

// registry maps SourceID to the corresponding adapterSpec.
// This is the single source of truth that handles all special cases.
//
// @MX:ANCHOR: [AUTO] Central SourceID→spec mapping; used by extract loop and tests
// @MX:REASON: fan_in >= 3 — extract(), TestExtract, gen-adapter-ref main() all read this
// @MX:SPEC: SPEC-DOC-002 REQ-ADPDOC-007
var registry = map[string]adapterSpec{
	"arxiv":      {pkgDir: "arxiv", primaryFile: "arxiv.go", funcName: ""},
	"github":     {pkgDir: "github", primaryFile: "github.go", funcName: ""},
	"hackernews": {pkgDir: "hn", primaryFile: "hn.go", funcName: ""},
	"koreanews":  {pkgDir: "koreanews", primaryFile: "koreanews.go", funcName: ""},
	"naver":      {pkgDir: "naver", primaryFile: "naver.go", funcName: ""},
	"reddit":     {pkgDir: "reddit", primaryFile: "reddit.go", funcName: ""},
	"searxng":    {pkgDir: "searxng", primaryFile: "searxng.go", funcName: ""},
	"youtube":    {pkgDir: "youtube", primaryFile: "youtube.go", funcName: ""},
	// social package special cases: bluesky and x are helpers inside social.go
	"bluesky": {pkgDir: "social", primaryFile: "social.go", funcName: "blueskyCapabilities"},
	"x":       {pkgDir: "social", primaryFile: "social.go", funcName: "xCapabilities"},
}

// extract parses the Go file at absPath and extracts the Capabilities literal
// from the function named funcName (package-level func) or, if funcName is "",
// from the Capabilities() method on any receiver.
// Returns a populated capabilitiesFields and the line number of the struct literal.
func extract(absPath, funcName string) (capabilitiesFields, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		return capabilitiesFields{}, err
	}

	var result capabilitiesFields
	var found bool
	var foundErr error

	ast.Inspect(f, func(n ast.Node) bool {
		if found || foundErr != nil {
			return false
		}
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		// Match: named package-level func (blueskyCapabilities, xCapabilities)
		// OR: receiver method named Capabilities() when funcName == "".
		isTarget := false
		if funcName != "" {
			isTarget = fn.Name.Name == funcName && fn.Recv == nil
		} else {
			isTarget = fn.Name.Name == "Capabilities" && fn.Recv != nil
		}
		if !isTarget {
			return true
		}

		// Walk the function body for the first ReturnStmt containing a
		// CompositeLit of types.Capabilities.
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			if found {
				return false
			}
			retStmt, ok := inner.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, retVal := range retStmt.Results {
				cl, ok := retVal.(*ast.CompositeLit)
				if !ok {
					continue
				}
				// Accept both qualified (types.Capabilities) and unqualified
				sel, ok := cl.Type.(*ast.SelectorExpr)
				if ok && sel.Sel.Name != "Capabilities" {
					continue
				}
				if !ok {
					ident, ok2 := cl.Type.(*ast.Ident)
					if !ok2 || ident.Name != "Capabilities" {
						continue
					}
				}
				// Extract the 5 target fields from the composite literal.
				var fields capabilitiesFields
				fields.SourceLine = fset.Position(cl.Pos()).Line
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					key, ok := kv.Key.(*ast.Ident)
					if !ok {
						continue
					}
					switch key.Name {
					case "SourceID":
						fields.SourceID = stringLitVal(kv.Value)
					case "RequiresAuth":
						fields.RequiresAuth = boolLitVal(kv.Value)
					case "AuthEnvVars":
						fields.AuthEnvVars = stringSliceLitVal(kv.Value)
					case "RateLimitPerMin":
						fields.RateLimitPerMin = intLitVal(kv.Value)
					case "DefaultMaxResults":
						fields.DefaultMaxResults = intLitVal(kv.Value)
					}
				}
				result = fields
				found = true
				return false
			}
			return true
		})
		return false
	})

	if foundErr != nil {
		return capabilitiesFields{}, foundErr
	}
	return result, nil
}

// stringLitVal returns the unquoted value of a basic string literal.
func stringLitVal(expr ast.Expr) string {
	bl, ok := expr.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return strings.Trim(bl.Value, `"`)
	}
	return s
}

// boolLitVal returns the bool value of an identifier (true/false).
func boolLitVal(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "true"
}

// intLitVal returns the integer value of a basic integer literal.
func intLitVal(expr ast.Expr) int {
	bl, ok := expr.(*ast.BasicLit)
	if !ok || bl.Kind != token.INT {
		return 0
	}
	v, err := strconv.Atoi(bl.Value)
	if err != nil {
		return 0
	}
	return v
}

// stringSliceLitVal extracts []string from a composite literal like
// []string{"A", "B"} or nil.
func stringSliceLitVal(expr ast.Expr) []string {
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var result []string
	for _, elt := range cl.Elts {
		s := stringLitVal(elt)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}
