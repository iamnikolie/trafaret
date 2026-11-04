package engine

import (
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

func TestCodexAgentRejectsBrokenFrontmatter(t *testing.T) {
	for name, in := range map[string]string{
		"none":         "just text",
		"unterminated": "---\nname: a\ndescription: d\n",
		"no name":      "---\ndescription: d\n---\nbody\n",
		"no desc":      "---\nname: a\n---\nbody\n",
	} {
		if _, err := codexAgent([]byte(in)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// A Codex project gets the readable contract and the native TOML, both derived
// from one render; a rendered agent that Codex cannot convert fails the render.
func TestCodexRenderIncludesReadableAndNativeAgentForms(t *testing.T) {
	e, _, _ := newConfig(t, "defaults:\n  workspace: {{WS}}\nprojects:\n  p:\n    repoRoot: p\n    harnesses: [codex]\n    agents: [a]\n", adapterFiles)
	got := renderEntry(t, e, "p")
	if got[".agents/agents/a.md"] == "" || !strings.Contains(got[".codex/agents/a.toml"], `name = "a"`) {
		t.Fatalf("Codex render lacks the readable or native form: %v", paths(got))
	}

	e, _, _ = newConfig(t, "defaults:\n  workspace: {{WS}}\nprojects:\n  p:\n    repoRoot: p\n    harnesses: [codex]\n    agents: [bad]\n", map[string]string{
		"templates/agents/bad.md": "no frontmatter at all\n",
	})
	if _, err := renderEntryErr(t, e, "p"); err == nil || !strings.Contains(err.Error(), `codex agent "bad"`) {
		t.Fatalf("a broken agent must name itself: %v", err)
	}
}
