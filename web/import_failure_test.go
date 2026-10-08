// story: BUG-001
package web_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

// lockedBuffer is a goroutine-safe log sink.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// captureLogs routes the default slog logger to a JSON buffer for one test.
func captureLogs(t *testing.T) *lockedBuffer {
	t.Helper()
	var logs lockedBuffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &logs
}

// importFailure returns the single "import failed" log record.
func importFailure(t *testing.T, logs *lockedBuffer) map[string]any {
	t.Helper()
	var found map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) == nil && rec["msg"] == "import failed" {
			if found != nil {
				t.Fatalf("more than one import failed record: %s", logs.String())
			}
			found = rec
		}
	}
	if found == nil {
		t.Fatalf("no import failed record in: %s", logs.String())
	}
	return found
}

func TestImportFlow_FailureShowsDetailAndLogsSafely(t *testing.T) {
	logs := captureLogs(t)
	b := newBrowser(t)
	signIn(t, b)
	itau := importBook(t, b)
	data, err := os.ReadFile("../ledger/importer/testdata/bradesco_dtserver_zero.ofx")
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(data), "<DTPOSTED>20260710000000[-03:EST]", "<DTPOSTED>00000000000000", 1)

	res, page := uploadCSV(t, b, itau, bad)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(page, "Detalhe técnico:") || !strings.Contains(page, "parse date") {
		t.Errorf("unreadable OFX = %d; want 400 showing the parser's technical detail", res.StatusCode)
	}
	rec := importFailure(t, logs)
	if rec["level"] != "WARN" || rec["account_id"] != itau || rec["format"] != "ofx" || rec["kind"] != "unreadable_ofx" || rec["user_id"] == nil {
		t.Errorf("log record = %v; want WARN with user_id, account_id, format ofx, kind unreadable_ofx", rec)
	}
	if s := logs.String(); strings.Contains(s, "parse date") || strings.Contains(s, "PADARIA") || strings.Contains(s, "45.90") {
		t.Errorf("log leaks file content or the parser message: %s", s)
	}
}

func TestImportFlow_FailureLogsRejectedLineCountOnly(t *testing.T) {
	logs := captureLogs(t)
	b := newBrowser(t)
	signIn(t, b)
	itau := importBook(t, b)
	csv := "id;data;hora;descricao;valor\n1;99/99/2026;;SEGREDO;-1,00\n2;01/01/2026;;OUTRO SEGREDO;abc\n"
	if res, _ := uploadCSV(t, b, itau, csv); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad CSV = %d, want 400", res.StatusCode)
	}
	rec := importFailure(t, logs)
	if rec["format"] != "csv" || rec["kind"] != "line_errors" || rec["rejected_lines"] != float64(2) {
		t.Errorf("log record = %v; want format csv, kind line_errors, rejected_lines 2", rec)
	}
	if strings.Contains(logs.String(), "SEGREDO") {
		t.Errorf("log leaks line content: %s", logs.String())
	}
}
