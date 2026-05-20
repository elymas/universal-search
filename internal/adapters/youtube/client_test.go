package youtube

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCategorizeStatusTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status  int
		wantCat string
	}{
		{200, ""}, // 200 is handled before categorizeStatus is called; verify Unknown
		{401, "permanent"},
		{403, "permanent"},
		{404, "permanent"},
		{429, "rate_limited"},
		{500, "unavailable"},
		{503, "unavailable"},
		{504, "unavailable"},
		{0, "unavailable"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			t.Parallel()
			se := categorizeStatus(tc.status, 30*time.Second, nil)
			if tc.wantCat == "" {
				return // status 200 tested separately
			}
			got := se.Category.String()
			if got != tc.wantCat {
				t.Errorf("categorizeStatus(%d).Category = %q, want %q", tc.status, got, tc.wantCat)
			}
			if se.HTTPStatus != tc.status {
				t.Errorf("categorizeStatus(%d).HTTPStatus = %d, want %d", tc.status, se.HTTPStatus, tc.status)
			}
		})
	}
}

func TestCategorizeStatusRateLimitedRetryAfter(t *testing.T) {
	t.Parallel()
	se := categorizeStatus(429, 45*time.Second, nil)
	if se.RetryAfter != 45*time.Second {
		t.Errorf("RetryAfter = %v, want 45s", se.RetryAfter)
	}
}

func TestParseRetryAfterTable(t *testing.T) {
	t.Parallel()
	now := time.Now()
	cases := []struct {
		name   string
		header string
		want   time.Duration
	}{
		{"integer-30s", "30", 30 * time.Second},
		{"integer-0", "0", defaultRetryAfter},
		{"integer-negative", "-5", defaultRetryAfter},
		{"integer-over-60-capped", "999", maxRetryAfter},
		{"missing", "", defaultRetryAfter},
		{"malformed", "not-a-date-or-int", defaultRetryAfter},
		{"http-date-future", now.Add(30 * time.Second).UTC().Format(http.TimeFormat), 0}, // tested separately
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.name == "http-date-future" {
				// HTTP-date test: check within (25s, 35s) window.
				header := now.Add(30 * time.Second).UTC().Format(http.TimeFormat)
				got := parseRetryAfter(header, now)
				if got < 25*time.Second || got > 35*time.Second {
					t.Errorf("HTTP-date Retry-After = %v, want in (25s, 35s)", got)
				}
				return
			}
			got := parseRetryAfter(tc.header, now)
			if got != tc.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}

func TestDoRequestSetsUserAgentAndAccept(t *testing.T) {
	t.Parallel()
	var capturedUA, capturedAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		capturedAccept = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	adapter, _ := New(Options{BaseURL: srv.URL})
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	resp, err := adapter.doRequest(req)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	resp.Body.Close()

	if capturedUA == "" {
		t.Error("User-Agent header not set")
	}
	if capturedAccept != "application/json" {
		t.Errorf("Accept header = %q, want %q", capturedAccept, "application/json")
	}
}

func TestDoRequestUserAgentVersionConfigurable(t *testing.T) {
	t.Parallel()
	var capturedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	adapter, _ := New(Options{BaseURL: srv.URL, UserAgentVersion: "v0.2-rc1"})
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/test", nil)
	resp, err := adapter.doRequest(req)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	resp.Body.Close()

	if capturedUA != "usearch/v0.2-rc1 (+https://github.com/elymas/universal-search)" {
		t.Errorf("User-Agent = %q, wrong version", capturedUA)
	}
}
