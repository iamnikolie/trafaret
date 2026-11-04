package engine

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexAgentConvertsMetadataAndInstructions(t *testing.T) {
	in := []byte(`---
name: reviewer
description: Reviews one change.
tools: Bash, Read, Grep
model: sonnet
---

Read the reviewer skill and report evidence.
`)

	got, err := codexAgent(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, want := range []string{
		`name = "reviewer"`,
		`description = "Reviews one change."`,
		`model = "gpt-5.6-terra"`,
		`model_reasoning_effort = "medium"`,
		`sandbox_mode = "read-only"`,
		`Read the reviewer skill and report evidence.`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("converted agent missing %q:\n%s", want, s)
		}
	}
}

func TestCodexAgentKeepsWritableAgentsWritable(t *testing.T) {
	in := []byte("---\nname: worker\ndescription: Implements a change.\ntools: Bash, Read, Edit, Write\nmodel: inherit\n---\n\nImplement it.\n")
	got, err := codexAgent(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "sandbox_mode") {
		t.Fatalf("writable agent unexpectedly constrained:\n%s", got)
	}
	if strings.Contains(string(got), "model =") {
		t.Fatalf("inheriting agent unexpectedly pins a model:\n%s", got)
	}
}

func TestCodexRenderIncludesReadableAndNativeAgentForms(t *testing.T) {
	p := testProject(t)
	files, err := render(p, testEngine(t), []string{"example"}, []string{"codex"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		filepath.Join(p.RepoRoot, ".agents", "agents", "example-reviewer.md"):  false,
		filepath.Join(p.RepoRoot, ".codex", "agents", "example-reviewer.toml"): false,
	}
	for _, f := range files {
		if _, ok := want[f.path]; ok {
			want[f.path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("Codex render missing %s", path)
		}
	}
}
