// story: e03s03
package web_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestPublicPages_RenderForEveryone(t *testing.T) {
	h := newHandler(t)
	for path, heading := range map[string]string{
		"/privacidade": "Política de Privacidade",
		"/termos":      "Termos de Uso",
	} {
		res, body := get(t, h, path)
		if res.StatusCode != http.StatusOK || !strings.Contains(body, heading) {
			t.Errorf("GET %s = %d, want 200 with %q", path, res.StatusCode, heading)
		}
		if !strings.Contains(body, "Texto provisório") {
			t.Errorf("%s does not flag its provisional text", path)
		}
	}
}

func TestPublicPages_FooterLinksOnEveryPage(t *testing.T) {
	h := newHandler(t)
	for _, path := range []string{"/", "/privacidade", "/termos"} {
		_, body := get(t, h, path)
		if !strings.Contains(body, `href="/privacidade"`) || !strings.Contains(body, `href="/termos"`) {
			t.Errorf("%s lacks footer links to /privacidade and /termos", path)
		}
	}
}
