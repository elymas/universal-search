package social

// Coverage for the cross-domain redirect error classifier.
//
// @MX:SPEC: SPEC-REL-001 — G1 release coverage gate

import (
	"errors"
	"testing"
)

func TestIsCrossDomainRedirectErr(t *testing.T) {
	if isCrossDomainRedirectErr(nil) {
		t.Error("nil must be false")
	}
	if isCrossDomainRedirectErr(errors.New("timeout reading body")) {
		t.Error("unrelated error must be false")
	}
	if !isCrossDomainRedirectErr(errors.New("bluesky: cross-domain redirect rejected: x")) {
		t.Error("matching error must be true")
	}
}
