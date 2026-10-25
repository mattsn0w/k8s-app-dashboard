package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNoPermissiveCORS asserts the API never sends a permissive
// Access-Control-Allow-Origin header, even when a cross-origin Origin is sent
// and even when a handler tries to set one. This pins the same-origin policy
// (SEC-07): no `*`, no reflected arbitrary origin.
func TestNoPermissiveCORS(t *testing.T) {
	// A handler that naively tries to "help" by setting a permissive CORS
	// header. The sameOriginOnly middleware must strip it.
	naive := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Vary", "Origin, Accept-Encoding")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	for name, h := range map[string]http.Handler{
		"naive-handler-strips": sameOriginOnly(naive),
		"plain-handler":        sameOriginOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("hi")) })),
	} {
		t.Run(name, func(t *testing.T) {
			for _, origin := range []string{"https://evil.example", "http://localhost:8080"} {
				req := httptest.NewRequest("GET", "/api/deployments", nil)
				req.Header.Set("Origin", origin)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
					t.Errorf("Access-Control-Allow-Origin = %q for Origin=%q, want empty (same-origin only)", got, origin)
				}
				if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
					t.Errorf("Access-Control-Allow-Credentials = %q, want empty", got)
				}
				if v := rec.Header().Get("Vary"); containsOrigin(v) {
					t.Errorf("Vary = %q still lists Origin, want it removed", v)
				}
			}
		})
	}
}

// containsOrigin reports whether a Vary header value lists Origin (any case).
func containsOrigin(vary string) bool {
	for _, p := range splitComma(vary) {
		if trimSpaceEQ(p, "origin") {
			return true
		}
	}
	return false
}

func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, trimSpace(cur))
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, trimSpace(cur))
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func trimSpaceEQ(s, target string) bool {
	a, b := []rune(s), []rune(target)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}
