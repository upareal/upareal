// story: e07s04
package importer

import (
	"bytes"
	"fmt"
	"io"
)

const sniffBytes = 4096

// Parse reads a statement in any supported format, detected by content:
// OFX when it starts with OFXHEADER or has <OFX> near the top, else the
// generic CSV.
func Parse(r io.Reader) ([]Line, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("importer: read: %w", err)
	}
	if len(data) > MaxBytes {
		return nil, ErrTooLarge
	}
	if isOFX(data) {
		return ParseOFX(bytes.NewReader(data))
	}
	return ParseGenericCSV(bytes.NewReader(data))
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
