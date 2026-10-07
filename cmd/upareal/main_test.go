// story: e01s01
package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	ln := listen(t)
	defer ln.Close()
	err := run(context.Background(), func(string) string { return "" }, ln, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "DB_PATH") {
		t.Fatalf("run without DB_PATH = %v, want error naming DB_PATH", err)
	}
}

func TestGracefulShutdown(t *testing.T) {
	ln := listen(t)
	url := "http://" + ln.Addr().String() + "/healthz"
	env := map[string]string{"DB_PATH": filepath.Join(t.TempDir(), "app.db")}
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
