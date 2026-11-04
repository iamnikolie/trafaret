package engine

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// update regenerates the golden files: go test ./... -run Golden -update
var update = flag.Bool("update", false, "regenerate the golden files under examples/demo/out")

const demoDir = "../../examples/demo"

// renderDemo renders every project and home of examples/demo in memory (the
// workspace is /work, nothing is written there; homes are mapped into a temp
// dir) and returns the files keyed "<entry>/<path as written>". The session
// hook's settings.json is included: it is part of what install writes.
func renderDemo(t *testing.T) map[string][]byte {
	t.Helper()
	e, err := New(Options{Config: demoDir, Version: "v0.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := e.LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	ns, err := e.Namespace()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]byte{}
	put := func(entry, base, path string, b []byte) {
		rel, err := filepath.Rel(base, path)
		if err != nil {
			t.Fatal(err)
		}
		got[entry+"/"+filepath.ToSlash(rel)] = b
	}

	for _, n := range sortedKeys(m.Projects) {
		p := m.Projects[n]
		active := p.ActiveBundles()
		it, err := e.projectItems(ns, p, active, true)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		files, err := ns.render(e.projectEntity(p, p.Harnesses), it)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		for _, f := range files {
			put(n, p.RepoRoot, f.path, f.bytes)
		}
		ctx, err := sessionContext(e.Bundles, active)
		if err != nil {
			t.Fatal(err)
		}
		if settings, _, err := planSessionHook(nil, false, ctx); err != nil {
			t.Fatal(err)
		} else if settings != nil {
			got[n+"/"+settingsFile] = settings
		}
	}

	tmp := t.TempDir()
	for _, n := range sortedKeys(m.Homes) {
		h := m.Homes[n]
		h.Dir = filepath.Join(tmp, n)
		it, err := e.homeItems(ns, h, h.Bundles, true)
		if err != nil {
			t.Fatalf("home %s: %v", n, err)
		}
		files, err := ns.render(homeEntity(h), it)
		if err != nil {
			t.Fatalf("home %s: %v", n, err)
		}
		for _, f := range files {
			put(n, h.Dir, f.path, f.bytes)
		}
	}
	return got
}

// TestGoldenDemo renders examples/demo and diffs it against examples/demo/out.
// The committed output is the documentation of what the templates do.
func TestGoldenDemo(t *testing.T) {
	got := renderDemo(t)
	outDir := filepath.Join(demoDir, "out")

	if *update {
		if err := os.RemoveAll(outDir); err != nil {
			t.Fatal(err)
		}
		for rel, b := range got {
			p := filepath.Join(outDir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			mode := os.FileMode(0o644)
			if bytes.HasPrefix(b, []byte("#!")) {
				mode = 0o755
			}
			if err := os.WriteFile(p, b, mode); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("wrote %d golden files under %s", len(got), outDir)
		return
	}

	want := map[string][]byte{}
	err := filepath.WalkDir(outDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(outDir, p)
		want[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatalf("read golden files: %v (regenerate with: go test ./... -run Golden -update)", err)
	}

	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		g, gok := got[k]
		w, wok := want[k]
		switch {
		case !wok:
			t.Errorf("%s: rendered but has no golden file", k)
		case !gok:
			t.Errorf("%s: golden file is no longer rendered", k)
		case !bytes.Equal(g, w):
			t.Errorf("%s differs from its golden file:\n%s", k, lineDiff(string(w), string(g)))
		}
	}
	if t.Failed() {
		t.Log("if the change is intended, regenerate with: go test ./... -run Golden -update")
	}
}

// lineDiff is a minimal diff: the first differing line with its neighbours.
func lineDiff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var a, b string
		if i < len(wl) {
			a = wl[i]
		}
		if i < len(gl) {
			b = gl[i]
		}
		if i >= len(wl) || i >= len(gl) || a != b {
			return fmt.Sprintf("  line %d\n  golden: %q\n  now:    %q", i+1, a, b)
		}
	}
	return "  (same lines, different bytes)"
}
