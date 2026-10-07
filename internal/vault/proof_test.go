// story: e02s01
package vault_test

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/upareal/upareal/internal/db"
)

// TestDatabaseCopyHasNoPlaintext proves the at-rest guarantee: a SQLite file
// written through the vault holds no readable personal text.
// Set UPAREAL_KEEP_DB=1 to keep the file for manual inspection (strings | grep).
func TestDatabaseCopyHasNoPlaintext(t *testing.T) {
	secrets := map[string]string{
		"name":        "Maria Silva",
		"email":       "maria.silva@example.com",
		"description": "Mercado Pão de Açúcar",
	}
	const control = "CONTROL-PLAINTEXT-MARKER" // stored unencrypted on purpose

	dir := t.TempDir()
	if os.Getenv("UPAREAL_KEEP_DB") == "1" {
		dir, _ = os.MkdirTemp("", "upareal-proof-")
		t.Logf("keeping database in %s", dir)
	}
	path := filepath.Join(dir, "proof.db")

	kr, _ := newKeyring(t)
	dk, wrapped, err := kr.NewDataKey("u1")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, conn, `CREATE TABLE people (
		id TEXT PRIMARY KEY, data_key BLOB, name BLOB, email BLOB, email_idx BLOB, description BLOB, control TEXT)`)
	sealed := map[string][]byte{}
	for field, value := range secrets {
		if sealed[field], err = dk.Seal(field, value); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(t, conn, `INSERT INTO people VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"u1", wrapped, sealed["name"], sealed["email"], kr.BlindIndex("email", secrets["email"]), sealed["description"], control)

	// The rows must really be there and decrypt, or the scan below proves nothing.
	var gotName []byte
	if err := conn.QueryRow(`SELECT name FROM people WHERE email_idx = ?`, kr.BlindIndex("email", secrets["email"])).Scan(&gotName); err != nil {
		t.Fatalf("lookup by blind index: %v", err)
	}
	restored, err := kr.UnwrapDataKey("u1", wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if name, err := restored.Open("name", gotName); err != nil || name != secrets["name"] {
		t.Fatalf("round trip through SQLite = %q, %v; want %q", name, err, secrets["name"])
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	raw := readDBFiles(t, path)
	if !bytes.Contains(raw, []byte(control)) {
		t.Fatal("control plaintext not found: the scan is not reading the stored data")
	}
	for field, value := range secrets {
		t.Logf("scanning for %s plaintext %q", field, value)
		if bytes.Contains(raw, []byte(value)) {
			t.Errorf("database files contain %s plaintext %q", field, value)
		}
	}
}

func mustExec(t *testing.T, conn *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

// readDBFiles concatenates the SQLite file and its -wal/-shm siblings, if present.
func readDBFiles(t *testing.T, path string) []byte {
	t.Helper()
	var all []byte
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue // -wal/-shm are removed on clean close
		}
		t.Logf("scanned %s (%d bytes)", filepath.Base(p), len(b))
		all = append(all, b...)
	}
	return all
}
