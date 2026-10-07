// story: e01s02
package main

import (
	"fmt"
	"net/http"
	"time"
)

const (
	healthcheckURL     = "http://127.0.0.1:8080/healthz"
	healthcheckTimeout = 3 * time.Second
)

// healthcheck lets the container check itself: distroless has no shell or curl.
// It returns nil only when url answers 200 within healthcheckTimeout.
func healthcheck(url string) error {
	client := &http.Client{Timeout: healthcheckTimeout}
	res, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck: GET %s: %w", url, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: GET %s = %d, want 200", url, res.StatusCode)
	}
	return nil
}
