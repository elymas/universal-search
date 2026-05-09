// Package synthcluster — Union-Find (Disjoint Set Union) for cluster assembly.
//
// SPEC-SYN-003 REQ-SYN3-002: candidate pairs assembled into clusters via
// Union-Find with path compression and union by rank.
package synthcluster

// unionFind implements a Union-Find data structure for N integer elements [0, N).
type unionFind struct {
	parent []int
	rank   []int
}

// newUnionFind creates a Union-Find with n elements, each in its own set.
func newUnionFind(n int) *unionFind {
	parent := make([]int, n)
	rank := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	return &unionFind{parent: parent, rank: rank}
}

// find returns the root of the set containing x, with path compression.
func (uf *unionFind) find(x int) int {
	for uf.parent[x] != x {
		// Path halving: compress path on the way up.
		uf.parent[x] = uf.parent[uf.parent[x]]
		x = uf.parent[x]
	}
	return x
}

// union merges the sets containing x and y. Returns true if they were in
// different sets (i.e., a new union was performed).
func (uf *unionFind) union(x, y int) bool {
	rx, ry := uf.find(x), uf.find(y)
	if rx == ry {
		return false
	}
	switch {
	case uf.rank[rx] < uf.rank[ry]:
		uf.parent[rx] = ry
	case uf.rank[rx] > uf.rank[ry]:
		uf.parent[ry] = rx
	default:
		uf.parent[ry] = rx
		uf.rank[rx]++
	}
	return true
}

// connected returns true if x and y are in the same set.
func (uf *unionFind) connected(x, y int) bool {
	return uf.find(x) == uf.find(y)
}

// groups returns a map from root index to the list of element indices in each group.
func (uf *unionFind) groups(n int) map[int][]int {
	m := make(map[int][]int, n)
	for i := 0; i < n; i++ {
		root := uf.find(i)
		m[root] = append(m[root], i)
	}
	return m
}
