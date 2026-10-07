// story: e01s01
package web_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestHomePage_RendersPtBRMobileLayout(t *testing.T) {
	res, body := get(t, newHandler(t), "/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	for _, want := range []string{
		`<html lang="pt-BR">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		`<script src="/static/htmx.min.js"`,
		`<link rel="stylesheet" href="/static/app.css">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("home page missing %s", want)
		}
	}
}

func TestHomePage_UnknownPathIs404(t *testing.T) {
	res, _ := get(t, newHandler(t), "/does-not-exist")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("GET /does-not-exist = %d, want 404", res.StatusCode)
	}
}

func TestStatic_ServesVendoredHTMX(t *testing.T) {
	res, body := get(t, newHandler(t), "/static/htmx.min.js")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /static/htmx.min.js = %d, want 200", res.StatusCode)
	}
	if len(body) < 40_000 || !strings.Contains(body, "htmx") {
		t.Errorf("htmx.min.js looks truncated or wrong: %d bytes", len(body))
	}
}

func TestStatic_CannotEscapeEmbeddedTree(t *testing.T) {
	res, _ := get(t, newHandler(t), "/static/../web.go")
	if res.StatusCode == http.StatusOK {
		t.Errorf("GET /static/../web.go = 200, want non-200")
	}
}
