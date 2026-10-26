package main

import (
	"crypto/sha1"
	"encoding/base64"
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

// TestStripCORSWriter_Hijackable asserts the CORS wrapper still satisfies
// http.Hijacker so the WebSocket log stream (handleLogsWS) can Hijack() through
// it. Regression test: SEC-07's middleware must not break WS upgrades.
func TestStripCORSWriter_Hijackable(t *testing.T) {
	// Use a real ResponseWriter that supports Hijack (httptest's recorder does
	// NOT, so use a hijacker-backed server via a real listener).
	// Simpler: assert the concrete type implements http.Hijacker.
	var _ http.Hijacker = &stripCORSWriter{ResponseWriter: httptest.NewRecorder()}
}

// TestSameOriginOnly_WSHandshake is an end-to-end regression test: a WebSocket
// upgrade must succeed through the sameOriginOnly middleware. Before the fix,
// stripCORSWriter did not expose Hijack(), so handleLogsWS returned 500
// "hijacking not supported" and the log view broke.
func TestSameOriginOnly_WSHandshake(t *testing.T) {
	// A minimal handler that performs the WS 101 handshake.
	wsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		key := r.Header.Get("Sec-WebSocket-Key")
		h := sha1.New()
		h.Write([]byte(key + wsGUID))
		accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
		resp := "HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\n" +
			"Connection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
		buf.WriteString(resp)
		buf.Flush()
	})

	srv := httptest.NewServer(sameOriginOnly(wsHandler))
	defer srv.Close()

	client := srv.Client()
	req, _ := http.NewRequest("GET", srv.URL+"/api/logs-ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101 Switching Protocols (WS upgrade must work through CORS middleware)", res.StatusCode)
	}
}
