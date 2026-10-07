// story: e03s01
package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCrossOrigin_RejectsCrossSiteUnsafeRequests(t *testing.T) {
	h := newHandler(t)
	for name, tc := range map[string]struct {
		method, header, value string
		wantForbidden         bool
	}{
		"cross-site POST (Sec-Fetch-Site)": {http.MethodPost, "Sec-Fetch-Site", "cross-site", true},
		"foreign Origin POST":              {http.MethodPost, "Origin", "https://evil.example", true},
		"same-origin POST":                 {http.MethodPost, "Sec-Fetch-Site", "same-origin", false},
		"cross-site GET is safe":           {http.MethodGet, "Sec-Fetch-Site", "cross-site", false},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set(tc.header, tc.value)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if got := rec.Code == http.StatusForbidden; got != tc.wantForbidden {
				t.Errorf("%s / = %d, want forbidden=%v", tc.method, rec.Code, tc.wantForbidden)
			}
		})
	}
}
