package engine

import (
	"fmt"
	"strings"
)

// --- harness adapters -------------------------------------------------------
//
// An adapter says where each kind of item lands for one harness, and in which
// form. The render, write, track and prune code is the same for every harness
// and for projects and homes alike; it only asks the adapter table. Adding a
// harness is adding one adapter value here — nothing else changes.

type scope int

const (
	projectScope scope = iota // a repository: paths are relative to its root
	homeScope                 // a harness user dir: paths are relative to the dir
)

// placement is one destination of an item.
type placement struct {
	// root is the template data's .TargetRoot for the render that feeds this
	// placement: the tree the file lands in, relative to the repo (".claude").
	// Empty in a home, where .TargetRoot is the absolute harness dir.
	root string
	// path is the destination relative to the repo or the home dir, with one %s
	// for the item name. For a skill it is the skill's directory, for an agent or
	// a rule the file.
	path string
	// convert turns the rendered Markdown into the harness's native form
	// (a Codex agent becomes TOML). nil keeps it as rendered.
	convert func([]byte) ([]byte, error)
	// aux marks a readable copy kept next to a native form (Codex's
	// .agents/agents/<name>.md). Auxiliary copies are written for rendered
	// agents only, not for a project's verbatim agent files.
	aux bool
}

// layout is what one harness does for one scope.
type layout struct {
	skills []placement
	agents []placement
	rules  []placement
}

// adapter describes one harness.
type adapter struct {
	name string
	// project and home are the layouts per scope. A kind left empty is not
	// supported there (Claude has no rules) and silently skipped.
	project layout
	home    layout
	// hook says the project's .claude/settings.json can carry the session hook
	// for this harness (it reads that file).
	hook bool
	// roots are the top-level project directories the harness writes into. They
	// are what `--target` partial installs own and what prune is allowed to touch.
	roots []string
}

var claudeProject = layout{
	skills: []placement{{root: ".claude", path: ".claude/skills/%s"}},
	agents: []placement{{root: ".claude", path: ".claude/agents/%s.md"}},
}

// adapters is the table, in the order harnesses render. Claude comes before
// Cursor on purpose: both write the shared .claude tree and the first to claim
// a path renders it.
var adapters = []adapter{
	{
		name:    "claude",
		project: claudeProject,
		home: layout{
			skills: []placement{{path: "skills/%s"}},
			agents: []placement{{path: "agents/%s.md"}},
		},
		hook:  true,
		roots: []string{".claude"},
	},
	{
		name: "codex",
		project: layout{
			skills: []placement{{root: ".agents", path: ".agents/skills/%s"}},
			// Codex loads the TOML; orchestration skills read the readable .md
			// when they need the full contract as orientation material.
			agents: []placement{
				{root: ".agents", path: ".agents/agents/%s.md", aux: true},
				{root: ".agents", path: ".codex/agents/%s.toml", convert: codexAgent},
			},
		},
		home: layout{
			skills: []placement{{path: "skills/%s"}},
			agents: []placement{{path: "agents/%s.toml", convert: codexAgent}},
		},
		roots: []string{".agents", ".codex"},
	},
	{
		name: "cursor",
		// Cursor reads .claude/skills and .claude/agents natively (its cross-agent
		// compatibility); a parallel .cursor copy would double-load every skill.
		// It contributes only its rules.
		project: layout{
			skills: claudeProject.skills,
			agents: claudeProject.agents,
			rules:  []placement{{root: ".cursor", path: ".cursor/rules/%s.mdc"}},
		},
		home: layout{
			skills: []placement{{path: "skills/%s"}},
			agents: []placement{{path: "agents/%s.md"}},
			rules:  []placement{{path: "rules/%s.mdc"}},
		},
		hook:  true,
		roots: []string{".cursor", ".claude"},
	},
}

func adapterNames() []string {
	out := make([]string, len(adapters))
	for i, a := range adapters {
		out[i] = a.name
	}
	return out
}

func findAdapter(name string) (adapter, bool) {
	for _, a := range adapters {
		if a.name == name {
			return a, true
		}
	}
	return adapter{}, false
}

func (a adapter) layout(s scope) layout {
	if s == homeScope {
		return a.home
	}
	return a.project
}

// checkHarnesses reports the first name that is not a known harness.
func checkHarnesses(names []string) error {
	for _, n := range names {
		if _, ok := findAdapter(n); !ok {
			return fmt.Errorf("unknown harness %q (want %s)", n, strings.Join(adapterNames(), "|"))
		}
	}
	return nil
}

// targetList expands a --target value into harness names: one harness, "both"
// (claude + cursor, the shared .claude tree) or "all".
func targetList(t string) ([]string, error) {
	switch t {
	case "both":
		return []string{"claude", "cursor"}, nil
	case "all":
		return adapterNames(), nil
	}
	if _, ok := findAdapter(t); ok {
		return []string{t}, nil
	}
	return nil, fmt.Errorf("invalid --target (%s|both|all): %s", strings.Join(adapterNames(), "|"), t)
}

// projectManagedRoots are the directories prune may ever delete under: the
// union of every adapter's roots.
func projectManagedRoots() []string {
	var out []string
	for _, a := range adapters {
		for _, r := range a.roots {
			if !contains(out, r) {
				out = append(out, r)
			}
		}
	}
	return out
}

// pathBelongsToTargets says whether a tracked repo-relative path belongs to a
// harness in targets — what keeps a partial `--target` install from pruning
// another harness's files.
func pathBelongsToTargets(rel string, targets []string) bool {
	top, _, _ := strings.Cut(rel, "/")
	for _, t := range targets {
		if a, ok := findAdapter(t); ok && contains(a.roots, top) {
			return true
		}
	}
	return false
}

// hasHook says whether any harness in the list reads the project settings hook.
func hasHook(harnesses []string) bool {
	for _, h := range harnesses {
		if a, ok := findAdapter(h); ok && a.hook {
			return true
		}
	}
	return false
}
