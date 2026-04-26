// Package router_test — shared test helpers.
package router_test

import "github.com/elymas/universal-search/pkg/types"

// queryText is a small helper that builds a types.Query whose only populated
// field is Text.
func queryText(s string) types.Query {
	return types.Query{Text: s}
}
