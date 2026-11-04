package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMergeVarsThreeLevels(t *testing.T) {
	cfg := map[string]any{"a": "cfg", "b": "cfg", "c": "cfg"}
	def := map[string]any{"b": "def", "c": "def"}
	own := map[string]any{"c": "own", "d": "own"}
	got := mergeVars(cfg, def, own)
	want := map[string]any{"a": "cfg", "b": "def", "c": "own", "d": "own"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeVarsNestedMapsMergeKeyByKey(t *testing.T) {
	def := map[string]any{"tracker": map[string]any{"kind": "github", "deep": map[string]any{"x": 1, "y": 2}}}
	own := map[string]any{"tracker": map[string]any{"repo": "acme/api", "deep": map[string]any{"y": 3}}}
	got := mergeVars(map[string]any{}, def, own)
	want := map[string]any{"tracker": map[string]any{"kind": "github", "repo": "acme/api", "deep": map[string]any{"x": 1, "y": 3}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeVarsScalarsAndListsReplace(t *testing.T) {
	def := map[string]any{
		"n":     1,
		"list":  []any{"a", "b", "c"},
		"gates": []any{map[string]any{"name": "t"}},
		"m":     map[string]any{"k": "v"},
		"s":     "scalar",
	}
	own := map[string]any{
		"n":     2,
		"list":  []any{"z"},
		"gates": []any{map[string]any{"cmd": "x"}},
		"m":     "now a scalar",
		"s":     map[string]any{"now": "a map"},
	}
	got := mergeVars(def, own)
	want := map[string]any{
		"n":     2,
		"list":  []any{"z"},
		"gates": []any{map[string]any{"cmd": "x"}}, // replaced, not merged element-wise
		"m":     "now a scalar",
		"s":     map[string]any{"now": "a map"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeVarsNullDeletesAKey(t *testing.T) {
	def := map[string]any{"e2e": "make e2e", "tracker": map[string]any{"kind": "github", "host": "h"}}
	own := map[string]any{"e2e": nil, "tracker": map[string]any{"host": nil}}
	got := mergeVars(def, own)
	if hasPath(got, "e2e") || hasPath(got, "tracker.host") || !hasPath(got, "tracker.kind") {
		t.Fatalf("null must delete exactly its key: %v", got)
	}
}

func TestMergeVarsSharesNothingWithItsInputs(t *testing.T) {
	def := map[string]any{"tracker": map[string]any{"kind": "github"}, "list": []any{map[string]any{"a": 1}}}
	a := mergeVars(def, map[string]any{"tracker": map[string]any{"repo": "a"}})
	b := mergeVars(def, map[string]any{"tracker": map[string]any{"repo": "b"}})
	a["tracker"].(map[string]any)["kind"] = "mutated"
	a["list"].([]any)[0].(map[string]any)["a"] = 99
	if def["tracker"].(map[string]any)["kind"] != "github" || def["list"].([]any)[0].(map[string]any)["a"] != 1 {
		t.Fatalf("the input was mutated through the merge result: %v", def)
	}
	if b["tracker"].(map[string]any)["kind"] != "github" || b["tracker"].(map[string]any)["repo"] != "b" {
		t.Fatalf("one entry's vars leaked into another's: %v", b)
	}
}

// The three layers come from the manifest: top-level vars < defaults.vars <
// the entry's own, for projects and homes alike; YAML ints stay ints.
func TestManifestVarsLayersForProjectsAndHomes(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.yaml")
	body := `vars: {company: Acme, reviewers: 1, tracker: {kind: none}}
defaults:
  vars: {branch: main, tracker: {kind: github}}
projects:
  one:
    repoRoot: /r/one
    vars: {tracker: {repo: acme/one}, branch: develop}
  two:
    repoRoot: /r/two
homes:
  h:
    harness: claude
    dir: /h
    vars: {company: Home Co}
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := loadManifest(p, "dev")
	if err != nil {
		t.Fatal(err)
	}
	one, two, h := m.Projects["one"].Vars, m.Projects["two"].Vars, m.Homes["h"].Vars
	if one["company"] != "Acme" || one["branch"] != "develop" || one["reviewers"] != 1 {
		t.Errorf("one: %v", one)
	}
	if !reflect.DeepEqual(one["tracker"], map[string]any{"kind": "github", "repo": "acme/one"}) {
		t.Errorf("one.tracker: %v", one["tracker"])
	}
	if two["branch"] != "main" || !reflect.DeepEqual(two["tracker"], map[string]any{"kind": "github"}) {
		t.Errorf("two: %v", two)
	}
	if h["company"] != "Home Co" || h["branch"] != "main" {
		t.Errorf("home: %v", h)
	}
}

func TestHasPath(t *testing.T) {
	vars := map[string]any{
		"empty":   "",
		"zero":    0,
		"flag":    false,
		"tickets": map[string]any{"db": "Work", "blank": "", "nested": map[string]any{"k": "v"}},
		"list":    []any{"a"},
	}
	cases := map[string]bool{
		"empty":             true, // an existing key is present even when its value is ""
		"zero":              true,
		"flag":              true,
		"tickets":           true,
		"tickets.db":        true,
		"tickets.blank":     true,
		"tickets.nested.k":  true,
		"missing":           false,
		"tickets.missing":   false,
		"tickets.db.deeper": false, // runs through a scalar
		"list.0":            false, // lists are not addressable
		"":                  false,
		"tickets.":          false,
	}
	for path, want := range cases {
		if got := hasPath(vars, path); got != want {
			t.Errorf("hasPath(%q) = %v, want %v", path, got, want)
		}
	}
	if hasPath(nil, "a") || hasPath("scalar", "a") {
		t.Error("has on a non-map is false")
	}
}

// has, index and default in real templates, including the optional-key idioms.
func TestTemplateOptionalValueIdioms(t *testing.T) {
	vars := "tracker: {kind: github}\nblank: \"\"\nname: Acme\nlist: [a, b]\nnothing: []"
	cases := []struct{ body, want string }{
		{`{{if has .Vars "blank"}}present{{else}}absent{{end}}`, "present"}, // "" counts as present
		{`{{if has .Vars "tracker.kind"}}y{{else}}n{{end}}`, "y"},
		{`{{if has .Vars "tracker.host"}}y{{else}}n{{end}}`, "n"},
		{`{{if has .Vars "nope.at.all"}}y{{else}}n{{end}}`, "n"},
		{`{{if index .Vars.tracker "host"}}y{{else}}n{{end}}`, "n"}, // index: absent key is falsy, no error
		{`{{if index .Vars.tracker "kind"}}y{{else}}n{{end}}`, "y"},
		{`{{if index .Vars "blank"}}y{{else}}n{{end}}`, "n"},
		{`{{default "x" (index .Vars "absent")}}`, "x"},
		{`{{default "x" .Vars.blank}}`, "x"}, // present but empty
		{`{{default "x" .Vars.name}}`, "Acme"},
		{`{{default "x" .Vars.nothing}}`, "x"}, // empty list
		{`{{join .Vars.list ", "}}`, "a, b"},
		{`{{dash "team memory"}}`, "team-memory"},
	}
	for _, c := range cases {
		got, err := tmplOut(t, vars, c.body)
		if err != nil {
			t.Errorf("%s: %v", c.body, err)
			continue
		}
		if strings.TrimSpace(got) != c.want {
			t.Errorf("%s = %q, want %q", c.body, strings.TrimSpace(got), c.want)
		}
	}
}

// A typo in a required path is a render error that names the key and the file;
// default does not rescue an absent key (the lookup fails first).
func TestMissingKeyIsARenderError(t *testing.T) {
	for _, body := range []string{
		`{{.Vars.tracker.knid}}`,
		`{{.Vars.nothing}}`,
		`{{default "x" .Vars.absent}}`,
	} {
		_, err := tmplOut(t, "tracker: {kind: github}", body)
		if err == nil || !strings.Contains(err.Error(), "map has no entry for key") || !strings.Contains(err.Error(), "skills/t/SKILL.md.tmpl") {
			t.Errorf("%s: want a missing-key error naming the file, got %v", body, err)
		}
	}
	if _, err := tmplOut(t, "a: 1", `{{.Nope}}`); err == nil {
		t.Error("an unknown data field must be a render error too")
	}
}
