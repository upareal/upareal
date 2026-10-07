// story: e01s01 e03s01
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upareal/upareal/internal/oidctest"
)

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

func TestRun_FailsFastWithoutDBPath(t *testing.T) {
	ln := listen(t) // run closes ln
	err := run(context.Background(), func(string) string { return "" }, ln, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "DB_PATH") {
		t.Fatalf("run without DB_PATH = %v, want error naming DB_PATH", err)
	}
}

func TestGracefulShutdown(t *testing.T) {
	ln := listen(t)
	url := "http://" + ln.Addr().String() + "/healthz"
	env := testEnv(t)
	var logs bytes.Buffer

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, func(k string) string { return env[k] }, ln, &logs) }()

	waitHealthy(t, url)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run after cancel = %v, want nil", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run did not return within 15s of cancel")
	}
	if _, err := http.Get(url); err == nil {
		t.Error("server still accepts requests after shutdown")
	}
	if !strings.Contains(logs.String(), `"msg":"shutdown complete"`) {
		t.Errorf("logs missing shutdown line: %s", logs.String())
	}
}

// testEnv returns a complete, valid configuration with a random master key.
func testEnv(t *testing.T) map[string]string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"DB_PATH":              filepath.Join(t.TempDir(), "app.db"),
		"MASTER_KEY":           base64.StdEncoding.EncodeToString(key),
		"GOOGLE_CLIENT_ID":     "test-client",
		"GOOGLE_CLIENT_SECRET": "test-secret",
		"BASE_URL":             "http://localhost:8080",
		"OIDC_ISSUER_URL":      oidctest.New(t, "test-client").URL,
	}
}

func waitHealthy(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if res, err := http.Get(url); err == nil {
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s never became healthy", url)
}
