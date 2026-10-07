// story: e01s01
package db_test

import (
	"path/filepath"
	"testing"

	"github.com/upareal/upareal/internal/db"
)

func TestOpen_AppliesPragmas(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	for pragma, want := range map[string]string{
		"journal_mode": "wal",
		"foreign_keys": "1",
		"busy_timeout": "5000",
	} {
		var got string
		if err := conn.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil {
			t.Fatalf("PRAGMA %s: %v", pragma, err)
		}
		if got != want {
			t.Errorf("PRAGMA %s = %q, want %q", pragma, got, want)
		}
	}
}

func TestOpen_FailsOnUnwritablePath(t *testing.T) {
	conn, err := db.Open(filepath.Join(t.TempDir(), "missing-dir", "test.db"))
	if err == nil {
		_ = conn.Close()
		t.Fatal("Open: expected error for a path in a missing directory, got nil")
	}
}
