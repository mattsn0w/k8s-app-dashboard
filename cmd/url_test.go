package main

import (
	"testing"
)

// ---------- isValidWorkloadURL ----------

func TestIsValidWorkloadURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		// Valid relative paths
		{name: "simple relative path", url: "/apps/echo", want: true},
		// Valid relative subpaths (no leading slash, end with "/") — used for
		// subpath-aware SPA workloads served under the dashboard's base path.
		{name: "relative subpath vscode/", url: "vscode/", want: true},
		{name: "relative subpath nested", url: "apps/echo/", want: true},
		{name: "relative subpath single word", url: "terminal/", want: true},
		{name: "root path (rejected by design)", url: "/", want: false},
		{name: "nested path", url: "/a/b/c/d", want: true},
		{name: "path with query", url: "/apps/echo?foo=bar", want: true},
		{name: "path with fragment", url: "/apps/echo#section", want: true},
		{name: "path with dot", url: "/v1.2/api", want: true},
		{name: "path with dash", url: "/my-app/endpoint", want: true},
		{name: "path with underscore", url: "/my_app/endpoint", want: true},
		{name: "path with port", url: "/:8080/api", want: true},
		{name: "path with trailing slash", url: "/apps/echo/", want: true},
		{name: "path with special chars", url: "/apps/echo?x=1&y=2", want: true},

		// Absolute http(s) URLs are REJECTED: workload-url must be same-origin
		// only (open-redirect fix, SEC-06). An attacker with annotation write
		// access must not be able to point the link at an external site.
		{name: "http URL rejected", url: "http://example.com", want: false},
		{name: "https URL rejected", url: "https://example.com", want: false},
		{name: "http URL with path rejected", url: "http://example.com/api", want: false},
		{name: "https URL with port rejected", url: "https://example.com:8080/api", want: false},
		{name: "http URL with query rejected", url: "http://example.com/api?x=1", want: false},
		{name: "https URL with subdomain rejected", url: "https://sub.example.com/api", want: false},
		{name: "http URL with credentials rejected", url: "http://user:pass@example.com", want: false},
		{name: "https URL with IP rejected", url: "https://192.168.1.1:8080/api", want: false},
		{name: "http URL with IP rejected", url: "http://10.0.0.1", want: false},

		// Protocol-relative URLs are REJECTED (would resolve against an external
		// host when the page is on a different origin).
		{name: "protocol-relative //host rejected", url: "//evil.example", want: false},
		{name: "protocol-relative //host/path rejected", url: "//evil.example/phish", want: false},
		{name: "just double slash rejected", url: "//", want: false},
		{name: "triple slash rejected", url: "///", want: false},

		// Invalid: empty
		{name: "empty string", url: "", want: false},

		// Invalid: whitespace
		{name: "whitespace only", url: "   ", want: false},
		{name: "leading whitespace", url: " /apps/echo", want: false},
		{name: "trailing whitespace", url: "/apps/echo ", want: false},
		{name: "internal whitespace", url: "/apps /echo", want: false},

		// Invalid: relative subpath without trailing slash, or with double slash
		{name: "relative no trailing slash", url: "vscode", want: false},
		{name: "relative double slash", url: "vscode//", want: false},
		{name: "relative just slash", url: "/", want: false},

		// Invalid: no scheme, no leading slash
		{name: "bare domain", url: "example.com", want: false},
		{name: "domain with path", url: "example.com/api", want: false},
		{name: "domain with port", url: "example.com:8080", want: false},
		{name: "just a word", url: "hello", want: false},
		{name: "just a slash char", url: "slash", want: false},

		// Invalid: bad scheme
		{name: "ftp scheme", url: "ftp://example.com", want: false},
		{name: "ws scheme", url: "ws://example.com", want: false},
		{name: "wss scheme", url: "wss://example.com", want: false},
		{name: "file scheme", url: "file:///etc/passwd", want: false},
		{name: "javascript scheme", url: "javascript:alert(1)", want: false},
		{name: "data scheme", url: "data:text/html,<script>alert(1)</script>", want: false},
		{name: "mailto scheme", url: "mailto:hello@example.com", want: false},

		// Invalid: scheme without //
		{name: "http without slashes", url: "http:example.com", want: false},
		{name: "https without slashes", url: "https:example.com", want: false},

		// Invalid: relative path with bad chars
		{name: "path with space", url: "/apps/echo server", want: false},
		{name: "path with newline", url: "/apps/echo\n", want: false},
		{name: "path with tab", url: "/apps/echo\t", want: false},

		// Edge cases
		{name: "path with @ (same-origin, allowed)", url: "/user@host/path", want: true},
		{name: "path with % (same-origin, allowed)", url: "/path%20with%20spaces", want: true},
		{name: "relative subpath with @ (same-origin, allowed)", url: "user@host/", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidWorkloadURL(tt.url)
			if got != tt.want {
				t.Errorf("isValidWorkloadURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsValidWorkloadURL_RejectsControlChars(t *testing.T) {
	controlChars := []string{
		"\x00", "\x01", "\x02", "\x03", "\x04", "\x05",
		"\x06", "\x07", "\x08", "\x0b", "\x0c",
		"\x0e", "\x0f", "\x10", "\x1f",
	}
	for _, c := range controlChars {
		t.Run(c, func(t *testing.T) {
			if isValidWorkloadURL("/apps/echo" + c) {
				t.Errorf("isValidWorkloadURL accepted control char %q", c)
			}
		})
	}
}
