// story: e07s04 BUG-001
package importer

import (
	"bytes"
	"fmt"
	"io"
)

const sniffBytes = 4096

// Parse reads a statement in any supported format, detected by content:
// OFX when it starts with OFXHEADER or has <OFX> near the top, else the
// generic CSV. format ("ofx" or "csv") is set even on error, for logging.
func Parse(r io.Reader) (lines []Line, format string, err error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("importer: read: %w", err)
	}
	if len(data) > MaxBytes {
		return nil, "", ErrTooLarge
	}
	if isOFX(data) {
		lines, err = ParseOFX(bytes.NewReader(data))
		return lines, "ofx", err
	}
	lines, err = ParseGenericCSV(bytes.NewReader(data))
	return lines, "csv", err
}

func isOFX(data []byte) bool {
	head := bytes.TrimSpace(bytes.TrimPrefix(data, []byte("\uFEFF")))
	if bytes.HasPrefix(head, []byte("OFXHEADER")) {
		return true
	}
	if len(head) > sniffBytes {
		head = head[:sniffBytes]
	}
	return bytes.Contains(head, []byte("<OFX>"))
}
