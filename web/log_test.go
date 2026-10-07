// story: e01s01
package web_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"testing"

	"github.com/upareal/upareal/web"
)

func TestRequestLog_WritesOneJSONLinePerRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	h := web.LogRequests(logger, newHandler(t))

	res, _ := get(t, h, "/healthz?secret=should-not-be-logged")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz = %d, want 200", res.StatusCode)
	}

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want 1: %s", len(lines), buf.String())
	}
	var entry map[string]any
	if err := json.Unmarshal(lines[0], &entry); err != nil {
		t.Fatalf("log line is not JSON: %v: %s", err, lines[0])
	}
	if entry["method"] != "GET" || entry["path"] != "/healthz" || entry["status"] != float64(200) {
		t.Errorf("log entry = %v, want method GET, path /healthz, status 200", entry)
	}
	if _, ok := entry["duration_ms"]; !ok {
		t.Errorf("log entry has no duration_ms: %v", entry)
	}
	if bytes.Contains(buf.Bytes(), []byte("should-not-be-logged")) {
		t.Errorf("query string leaked into the log: %s", buf.String())
	}
}

func TestRequestLog_RecordsErrorStatus(t *testing.T) {
	var buf bytes.Buffer
	h := web.LogRequests(slog.New(slog.NewJSONHandler(&buf, nil)), newHandler(t))
	get(t, h, "/missing")

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if entry["status"] != float64(404) {
		t.Errorf("status = %v, want 404", entry["status"])
	}
}
