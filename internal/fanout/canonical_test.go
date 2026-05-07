package fanout

import "testing"

// TestCanonicalURLTable verifies all 8 normalisation rules from SPEC-FAN-001 §2.4.
func TestCanonicalURLTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    string // empty means error expected
		wantErr bool
	}{
		{
			name:  "Rule1: uppercase scheme is lowercased",
			input: "HTTP://example.com/path",
			want:  "http://example.com/path",
		},
		{
			name:  "Rule2: uppercase host is lowercased",
			input: "https://EXAMPLE.COM/path",
			want:  "https://example.com/path",
		},
		{
			name:  "Rule3: fragment is stripped",
			input: "https://example.com/path?q=1#section",
			want:  "https://example.com/path?q=1",
		},
		{
			name:  "Rule4: tracking params are stripped",
			input: "https://example.com/page?utm_source=google&utm_medium=cpc&q=hello",
			want:  "https://example.com/page?q=hello",
		},
		{
			name:  "Rule4: all 12 tracking params stripped",
			input: "https://example.com/?utm_source=a&utm_medium=b&utm_campaign=c&utm_term=d&utm_content=e&gclid=f&fbclid=g&mc_eid=h&mc_cid=i&_ga=j&ref=k&ref_src=l&keep=1",
			want:  "https://example.com/?keep=1",
		},
		{
			name:  "Rule5: trailing slash stripped from path",
			input: "https://example.com/path/",
			want:  "https://example.com/path",
		},
		{
			name:  "Rule5: root slash preserved",
			input: "https://example.com/",
			want:  "https://example.com/",
		},
		{
			name:  "Rule6: query params sorted alphabetically",
			input: "https://example.com/search?z=last&a=first&m=middle",
			want:  "https://example.com/search?a=first&m=middle&z=last",
		},
		{
			name:  "Rules combined: scheme+host lowercase + tracking stripped + sorted",
			input: "HTTPS://Example.COM/PAGE/?utm_source=x&b=2&a=1#frag",
			want:  "https://example.com/PAGE?a=1&b=2",
		},
		{
			name:    "Error: missing scheme",
			input:   "//example.com/path",
			wantErr: true,
		},
		{
			name:    "Error: missing host",
			input:   "https:///path",
			wantErr: true,
		},
		{
			name:    "Error: empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:  "Rule4+Rule6: tracking stripped then remaining sorted",
			input: "https://example.com/?z=3&utm_campaign=x&a=1",
			want:  "https://example.com/?a=1&z=3",
		},
		{
			name:  "Rule3+Rule6: fragment stripped, params sorted",
			input: "https://example.com/p?c=3&a=1&b=2#anchor",
			want:  "https://example.com/p?a=1&b=2&c=3",
		},
		{
			name:  "No query string: no trailing slash on path segment",
			input: "https://example.com/foo/bar/",
			want:  "https://example.com/foo/bar",
		},
		{
			name:  "Preserves path case (Rule 7)",
			input: "https://example.com/PATH/To/File",
			want:  "https://example.com/PATH/To/File",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("canonicalURL(%q) = %q, nil; want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("canonicalURL(%q) error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("canonicalURL(%q)\n  got  = %q\n  want = %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestCanonicalURLDeterministic verifies same input → same output on repeated calls.
func TestCanonicalURLDeterministic(t *testing.T) {
	t.Parallel()
	const raw = "HTTPS://Example.COM/path/?z=3&utm_source=x&a=1#frag"
	first, err := canonicalURL(raw)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	for i := range 10 {
		got, err := canonicalURL(raw)
		if err != nil {
			t.Fatalf("call %d error: %v", i, err)
		}
		if got != first {
			t.Fatalf("non-deterministic: call 0=%q, call %d=%q", first, i, got)
		}
	}
}
