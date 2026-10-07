// story: e01s02
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthcheckCommand(t *testing.T) {
	for name, tc := range map[string]struct {
		status  int
		wantErr bool
	}{
		"healthy":     {http.StatusOK, false},
		"unavailable": {http.StatusServiceUnavailable, true},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()
			if err := healthcheck(srv.URL + "/healthz"); (err != nil) != tc.wantErr {
				t.Errorf("healthcheck(%d) error = %v, wantErr %v", tc.status, err, tc.wantErr)
			}
		})
	}

	t.Run("unreachable", func(t *testing.T) {
		srv := httptest.NewServer(http.NotFoundHandler())
		url := srv.URL + "/healthz"
		srv.Close()
		if err := healthcheck(url); err == nil {
			t.Error("healthcheck on a closed server = nil, want error")
		}
	})
}
