// story: e12s03
package web_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestBrand_FaviconLogoAndHeadLinks(t *testing.T) {
	b := newBrowser(t)

	res, ico := b.get(t, "/favicon.ico")
	if res.StatusCode != http.StatusOK || !bytes.HasPrefix([]byte(ico), []byte{0, 0, 1, 0}) {
		t.Errorf("/favicon.ico = %d, %d bytes; want 200 with an ICO", res.StatusCode, len(ico))
	}
	for _, path := range []string{"/static/logo.png", "/static/apple-touch-icon.png"} {
		res, body := b.get(t, path)
		if res.StatusCode != http.StatusOK || !strings.HasPrefix(body, "\x89PNG") {
			t.Errorf("%s = %d; want 200 with a PNG", path, res.StatusCode)
		}
	}
	_, home := b.get(t, "/")
	for _, want := range []string{
		`<link rel="icon" href="/favicon.ico" sizes="any">`,
		`<link rel="apple-touch-icon" href="/static/apple-touch-icon.png">`,
		`<img src="/static/logo.png"`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("home page missing %s", want)
		}
	}
	if _, terms := b.get(t, "/termos"); !strings.Contains(terms, `rel="icon"`) {
		t.Error("the favicon link is not on every page")
	}
}
