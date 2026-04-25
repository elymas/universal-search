// Package types_test — benchmarks for NFR-CORE-001 performance gates.
package types_test

import (
	"testing"
)

// BenchmarkNormalizedDocValidate measures Validate() throughput on a fully-
// populated doc. NFR-CORE-001 requires < 1 µs/op on amd64 in CI.
func BenchmarkNormalizedDocValidate(b *testing.B) {
	d := fullyPopulatedDoc()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := d.Validate(); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

// BenchmarkNormalizedDocCanonicalHash measures CanonicalHash() throughput.
// NFR-CORE-001 requires < 5 µs/op on amd64 in CI.
func BenchmarkNormalizedDocCanonicalHash(b *testing.B) {
	d := fullyPopulatedDoc()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.CanonicalHash()
	}
}
