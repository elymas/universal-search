// Package social — tests for AT-URI parsing and Bluesky URL construction.
// REQ-ADP6-006: AT-URI -> bsky.app URL mapping.
package social

import (
	"testing"
)

// TestParseATURITable verifies AT-URI rkey extraction.
func TestParseATURITable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		uri     string
		wantDid string
		wantKey string
		wantErr bool
	}{
		{
			name:    "standard AT-URI with did:plc",
			uri:     "at://did:plc:user001/app.bsky.feed.post/3abc0000001aa",
			wantDid: "did:plc:user001",
			wantKey: "3abc0000001aa",
			wantErr: false,
		},
		{
			name:    "AT-URI with handle-style author",
			uri:     "at://alice.bsky.social/app.bsky.feed.post/3abc0000001aa",
			wantDid: "alice.bsky.social",
			wantKey: "3abc0000001aa",
			wantErr: false,
		},
		{
			name:    "high-engagement test post rkey",
			uri:     "at://did:plc:viraluser/app.bsky.feed.post/3viral00001aa",
			wantDid: "did:plc:viraluser",
			wantKey: "3viral00001aa",
			wantErr: false,
		},
		{
			name:    "empty string returns error",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "missing at:// prefix returns error",
			uri:     "did:plc:user001/app.bsky.feed.post/3abc",
			wantErr: true,
		},
		{
			name:    "too few path segments returns error",
			uri:     "at://did:plc:user001/app.bsky.feed.post",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			did, key, err := parseATURI(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseATURI(%q): expected error, got nil (did=%q, key=%q)", tc.uri, did, key)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseATURI(%q): unexpected error: %v", tc.uri, err)
			}
			if did != tc.wantDid {
				t.Errorf("parseATURI(%q) did: got %q, want %q", tc.uri, did, tc.wantDid)
			}
			if key != tc.wantKey {
				t.Errorf("parseATURI(%q) key: got %q, want %q", tc.uri, key, tc.wantKey)
			}
		})
	}
}

// TestConstructBlueskyURLTable verifies bsky.app URL construction.
func TestConstructBlueskyURLTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		handle string
		rkey   string
		want   string
	}{
		{
			name:   "standard handle and rkey",
			handle: "alice.bsky.social",
			rkey:   "3abc0000001aa",
			want:   "https://bsky.app/profile/alice.bsky.social/post/3abc0000001aa",
		},
		{
			name:   "did:plc handle",
			handle: "did:plc:user001",
			rkey:   "3abc0000001aa",
			want:   "https://bsky.app/profile/did:plc:user001/post/3abc0000001aa",
		},
		{
			name:   "viral user",
			handle: "viral.bsky.social",
			rkey:   "3viral00001aa",
			want:   "https://bsky.app/profile/viral.bsky.social/post/3viral00001aa",
		},
		{
			name:   "paginator user",
			handle: "paginator.bsky.social",
			rkey:   "3pag0000001aa",
			want:   "https://bsky.app/profile/paginator.bsky.social/post/3pag0000001aa",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := constructBlueskyURL(tc.handle, tc.rkey)
			if got != tc.want {
				t.Errorf("constructBlueskyURL(%q, %q): got %q, want %q", tc.handle, tc.rkey, got, tc.want)
			}
		})
	}
}
