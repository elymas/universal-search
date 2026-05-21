// Package synthcluster — SimHash computation utilities.
//
// SPEC-SYN-003 REQ-SYN3-001: 64-bit Charikar SimHash over character 3-shingles,
// NFC-normalized. Algorithm choice: character 3-shingles for v0 (no mecab-ko
// dependency); mecab-ko upgrade gated on SPEC-EVAL-003 recall measurements.
//
// @MX:NOTE: [AUTO] SimHash algorithm: character 3-shingles over NFC-normalized text.
// Korean word boundaries are not used in v0 (mecab-ko deferred to SPEC-EVAL-003).
// HammingThreshold=4 follows Manku et al. (WWW 2007) web-scale near-dup empirics.
package synthcluster

import (
	"encoding/binary"
	"encoding/hex"
	"math/bits"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// SimHash64 computes a 64-bit Charikar SimHash over the NFC-normalized text.
// Token source: character 3-shingles (consecutive rune triples).
// Hash function for shingles: FNV-1a 64-bit.
//
// The function is deterministic: byte-identical inputs always produce the same
// output within a single binary (REQ-SYN3-001).
//
// @MX:ANCHOR: [AUTO] SimHash entry point; callers: Cluster, tests, benchmarks
// @MX:REASON: fan_in >= 3; every near-dup detection path flows through this function
func SimHash64(text string) uint64 {
	// NFC normalization per Unicode UAX #15 (SPEC-SYN-003 REQ-SYN3-001).
	normalized := norm.NFC.String(text)

	// Accumulate bit-position votes.
	// votes[i] > 0 → final bit i is 1; votes[i] <= 0 → bit i is 0.
	var votes [64]int

	// Iterate over character 3-shingles.
	runes := []rune(normalized)
	for i := 0; i+3 <= len(runes); i++ {
		shingle := string(runes[i : i+3])
		h := fnv64a(shingle)
		for bit := 0; bit < 64; bit++ {
			if (h>>uint(bit))&1 == 1 {
				votes[bit]++
			} else {
				votes[bit]--
			}
		}
	}

	// Handle texts with fewer than 3 runes by treating the whole text as one token.
	if utf8.RuneCountInString(normalized) < 3 {
		h := fnv64a(normalized)
		for bit := 0; bit < 64; bit++ {
			if (h>>uint(bit))&1 == 1 {
				votes[bit]++
			} else {
				votes[bit]--
			}
		}
	}

	var fingerprint uint64
	for bit := 0; bit < 64; bit++ {
		if votes[bit] > 0 {
			fingerprint |= 1 << uint(bit)
		}
	}
	return fingerprint
}

// fnv64a computes FNV-1a 64-bit hash of a string.
// Used as the per-shingle hash function inside SimHash64.
func fnv64a(s string) uint64 {
	const (
		offset uint64 = 14695981039346656037
		prime  uint64 = 1099511628211
	)
	h := offset
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime
	}
	return h
}

// HammingDistance returns the number of differing bits between a and b.
//
// @MX:ANCHOR: [AUTO] Hamming distance used in candidate-pair detection; callers: Cluster, tests, benchmarks
// @MX:REASON: fan_in >= 3; all O(N^2) pair-filter calls route through here
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// SimHashHex returns the 16-character lowercase hex representation of a SimHash digest.
// Used in Metadata["spec_syn003_cluster"]["simhash"] field.
func SimHashHex(h uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], h)
	return hex.EncodeToString(buf[:])
}
