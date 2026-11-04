package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The repository is public: no deployment-specific name, host, path or address
// may appear in the sources, templates or docs. Values a template needs come from
// manifest tokens. (Patterns are assembled from parts so this file does not trip
// its own check.) The scan covers the whole tree: cmd/, internal/, examples/,
// docs/, .github/ and the top-level files.
func TestNoPrivateStringsInSources(t *testing.T) {
	forbidden := []string{
		"lang" + "gerone", "ma" + "ko", "/Users" + "/", "imn" + "dev", "mb" + "dev",
		"fibery" + ".io", "@" + "gmail", "iamnikolie" + ".info", "imn-" + "ao", "lg-" + "ao",
	}
	// The module path names the publisher: Go imports, go.mod, the Makefile's
	// ldflags and the links in the README and the contributor docs. It is the
	// only spelling allowed, and only in these files.
	publisher := "iam" + "nikolie"
	modulePath := "github.com/" + publisher + "/trafaret"
	mayNamePublisher := func(name string) bool {
		switch name {
		case "go.mod", "Makefile", "README.md", "CONTRIBUTING.md", "SECURITY.md":
			return true
		}
		return strings.HasSuffix(name, ".go")
	}
	self := "main_test.go"
	var scan func(dir string)
	scan = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			switch {
			case e.IsDir():
				if e.Name() == ".git" {
					continue
				}
				scan(path)
			// .goreleaser.yaml publishes to the owner's Homebrew tap and names the
			// formula commit author by design; it is release plumbing, not engine.
			// mermaid.min.js is a vendored third-party bundle.
			case e.Name() == self, e.Name() == "trafaret", e.Name() == ".goreleaser.yaml", e.Name() == "mermaid.min.js", strings.HasSuffix(e.Name(), ".sum"):
			default:
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				low := strings.ToLower(string(b))
				for _, f := range forbidden {
					if strings.Contains(low, f) {
						t.Errorf("%s contains private string %q", path, f)
					}
				}
				if mayNamePublisher(e.Name()) {
					low = strings.ReplaceAll(low, modulePath, "")
				}
				if strings.Contains(low, publisher) {
					t.Errorf("%s names the publisher; only the module path may, and only in Go sources, go.mod, the Makefile, the README and the contributor docs", path)
				}
			}
		}
	}
	scan(".")
}
