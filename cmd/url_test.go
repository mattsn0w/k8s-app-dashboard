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

		// Valid absolute URLs
		{name: "http URL", url: "http://example.com", want: true},
		{name: "https URL", url: "https://example.com", want: true},
		{name: "http URL with path", url: "http://example.com/api", want: true},
		{name: "https URL with port", url: "https://example.com:8080/api", want: true},
		{name: "http URL with query", url: "http://example.com/api?x=1", want: true},
		{name: "https URL with subdomain", url: "https://sub.example.com/api", want: true},

		// Invalid: empty
		{name: "empty string", url: "", want: false},

		// Invalid: whitespace
		{name: "whitespace only", url: "   ", want: false},
		{name: "leading whitespace", url: " /apps/echo", want: false},
		{name: "trailing whitespace", url: "/apps/echo ", want: false},
		{name: "internal whitespace", url: "/apps /echo", want: false},

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
		{name: "just double slash", url: "//", want: true},
		{name: "triple slash", url: "///", want: true},
		{name: "path with @", url: "/user@host/path", want: true},
		{name: "path with %", url: "/path%20with%20spaces", want: true},
		{name: "http with @", url: "http://user:pass@example.com", want: true},
		{name: "https with IP", url: "https://192.168.1.1:8080/api", want: true},
		{name: "http with IP", url: "http://10.0.0.1", want: true},
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
