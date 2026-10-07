// story: e03s02
package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func requestFrom(h http.Handler, ip, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = ip + ":40000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLoginRateLimit_TenPerMinutePerIP(t *testing.T) {
	h := newHandler(t)
	for i := 1; i <= 10; i++ {
		if rec := requestFrom(h, "203.0.113.1", "/auth/login"); rec.Code != http.StatusFound {
			t.Fatalf("request %d = %d, want 302 (within limit)", i, rec.Code)
		}
	}
	rec := requestFrom(h, "203.0.113.1", "/auth/login")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "Muitas tentativas") {
		t.Errorf("11th request = %d, want 429 with pt-BR message", rec.Code)
	}
	if rec := requestFrom(h, "203.0.113.1", "/auth/callback?state=x&code=y"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("callback from the same IP = %d, want 429 (shared /auth limit)", rec.Code)
	}
	if rec := requestFrom(h, "198.51.100.7", "/auth/login"); rec.Code != http.StatusFound {
		t.Errorf("another IP = %d, want 302 (own limit)", rec.Code)
	}
	if rec := requestFrom(h, "203.0.113.1", "/"); rec.Code != http.StatusOK {
		t.Errorf("home from limited IP = %d, want 200 (only /auth is limited)", rec.Code)
	}
}
