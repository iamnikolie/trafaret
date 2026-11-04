package site

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/iamnikolie/trafaret/internal/engine"
)

// The operator site is bilingual: every key must exist in both maps, and every
// key the skeleton references must exist at all. This is the enforcement behind
// the "agent edits = site edits" contract — prose drift fails the build.

func TestSiteStringKeysParity(t *testing.T) {
	for k := range siteEN {
		if _, ok := siteRU[k]; !ok {
			t.Errorf("key %q exists in siteEN but is missing in siteRU", k)
		}
	}
	for k := range siteRU {
		if _, ok := siteEN[k]; !ok {
			t.Errorf("key %q exists in siteRU but is missing in siteEN", k)
		}
	}
}

func TestSkeletonReferencesExistingKeys(t *testing.T) {
	re := regexp.MustCompile(`\{\{\$?\.T\.([A-Za-z0-9_]+)\}\}`)
	for _, m := range re.FindAllStringSubmatch(siteSkeleton, -1) {
		if _, ok := siteEN[m[1]]; !ok {
			t.Errorf("skeleton references key %q not present in siteEN", m[1])
		}
	}
}

// The handler serves every page in both languages, the vendored mermaid script
// and a redirect from a bare language; anything else is a 404.
func TestHandlerServesPagesInBothLanguages(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("projects: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(engine.Options{Config: dir})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(eng))
	defer srv.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	status := func(path string) (int, string) {
		t.Helper()
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	for _, lang := range []string{"en", "ru"} {
		for _, sp := range sitePages {
			if code, body := status("/" + lang + "/" + sp.Slug); code != http.StatusOK || !strings.Contains(body, `lang="`+lang+`"`) {
				t.Errorf("/%s/%s: status %d, lang attribute present: %v", lang, sp.Slug, code, strings.Contains(body, `lang="`+lang+`"`))
			}
		}
		if code, _ := status("/" + lang); code != http.StatusFound {
			t.Errorf("/%s must redirect to its first page, got %d", lang, code)
		}
	}
	if code, body := status("/mermaid.min.js"); code != http.StatusOK || len(body) == 0 {
		t.Errorf("mermaid.min.js: status %d, %d bytes", code, len(body))
	}
	if code, _ := status("/"); code != http.StatusOK {
		t.Errorf("language index: status %d", code)
	}
	for _, path := range []string{"/de/overview", "/en/nope", "/en/overview/extra"} {
		if code, _ := status(path); code != http.StatusNotFound {
			t.Errorf("%s: want 404, got %d", path, code)
		}
	}
}
