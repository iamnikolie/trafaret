# trafaret — design & rationale (v0.3)

## Problem

Agent skills, subagents and rules are valuable, but written once for one project they are hardcoded to that
project's ticket board, docs paths, gate matrix and stack conventions. Copy-pasted into other repos they drift: the
ticket cheat-sheet differs by workspace name only, yet each copy ages separately, and only the repo that happened to
get the files has them at all. The same library is also wanted in the user's own harness directories (`~/.claude`,
`~/.codex`, `~/.cursor`), and each harness (Claude Code, Codex, Cursor) wants its files in its own place and its own
format: a Markdown agent for one, TOML for another, an `.mdc` rule for a third.

Version 0.2 solved the per-project half with typed project fields (`tickets`, `gates`, `docs`, `vcs`, …) and tokens
derived from them, and handled user-level directories through a separate verbatim copy. That tied the engine to one
process: any team with a different ticket system, gate shape or docs layout needed an engine release, and a skill
could not be both parameterised and installed into a home.

## Goal

Write a skill, agent or rule once; render it, with values inserted from `vars`, into every harness that should have
it — per-repo **projects** and user-level **homes** alike — through one pipeline. Kill the duplication.

trafaret is the **engine**: it renders, lints, writes and prunes. It knows harness layouts and nothing else — no
ticket system, no process, no stack. It does not ship a library. Its binary embeds a small example set that shows the
template format; the real templates belong to whoever runs it and live in their own config directory.

## Positioning

- **Not a library.** The embedded examples (`examples/`, with a runnable demo in `examples/demo`) exist to show the
  format. A skill collection is the user's own content.
- **Not a process engine.** v0.2 modelled tickets, gates and docs; v0.3 models none of them. Every project fact is a
  free value under `vars:` that a template reads as `.Vars.<key>`. The engine never interprets it.
- **Not a config bundler.** Settings, statusline and memory sync have a different lifecycle and different owners; this
  tool stays with skills, agents and rules (plus the one session hook that makes a skill's context load).
- **One pipeline.** A project and a home differ only in where the adapter puts the files; render, lint, write, track
  and prune are the same code. A new harness is an adapter value, not a new code path.

## Approach (chosen)

**Templated install.** Generic Go `text/template` sources plus `vars` in a `manifest.yaml`;
`trafaret install <name>` renders concrete, self-contained artifacts. No runtime indirection (an unattended agent
must not depend on a model remembering to read a profile mid-run): the rendered files are fully concrete.

Alternatives rejected:

- _Runtime profile_ (generic skills plus a profile document read at runtime) — a reliability risk for an unattended
  agent, and a copy mechanism is needed regardless (skills must physically live in `.claude/skills/`; symlinks break
  under worktrees).
- _Typed project fields with derived tokens_ (the v0.2 design) — every new need (a different tracker, a gate shape, a
  docs layout) meant a new field, new derived tokens and an engine release, and the engine ended up modelling one
  team's process. Free `vars` plus partials move that logic into the templates, where it is maintained.
- _Separate pipelines for homes and projects_ (v0.2 copied home skills verbatim, with no parameters) — the same skill
  could not be rendered for a home, and two code paths drifted. One pipeline with an adapter table removes both.
- _A ticket-system registry or showcase_ — pays off for broad team distribution; for a handful of repos it is pure
  overhead next to a manifest.
- _Config-as-code bundle_ (settings, statusline, memory sync in the same tool) — different lifecycle and different
  owners; keeping this tool to skills, agents and rules keeps it small.

## Architecture

```
embedded examples ──┐
<config>/templates ─┤   skills, agents, rules, partials, bundles.yaml (overlay: same path wins)
<config>/global, agents/<role>/skills ─┤   more skills, same namespace
<config>/projects/<name>/{skills,agents} ─┤   verbatim, per project
<config>/manifest.yaml ─┘   vars, projects, homes
        │   render (per harness)  →  lint  →  write (sha256-idempotent)  →  track  →  prune
        ▼
project:  <repoRoot>/.claude  .agents  .codex  .cursor
home:     <dir>/skills  agents  rules
```

- **Engine and config are separate.** The binary embeds the example template set (`go:embed`, served through
  `io/fs`); everything that belongs to the user lives in a **config directory**, the directory holding
  `manifest.yaml`. It is chosen by `--manifest`, then `--config`, then `$TRAFARET_CONFIG`, then by walking up from the
  working directory. One binary serves any number of independent configs (a personal one, a team one).
- **Templates overlay.** `<config>/templates/<path>` replaces the embedded file with the same path and may add files;
  directories merge. A config's real templates are simply files in this overlay, so adding or changing one never means
  forking the engine.
- **Adapters.** A small table says, per harness and per scope (project or home), where each kind of item lands and in
  which form. See Harness adapters.
- **Data-driven bundles.** A bundle is a named set of skills, agents and rules; the registry is data. See Bundles.
- **Engine gate.** An `engine:` constraint in the manifest makes a config refuse to run on a binary that is too old.

## Sources and namespace

```
manifest.yaml
templates/
  bundles.yaml              optional named groups of skills, agents and rules (shadows the embedded registry)
  skills/<name>/...         skill sources
  agents/<name>.md[.tmpl]   subagent sources (frontmatter: name, description, tools, model)
  rules/<name>.md[.tmpl]    always-on rule sources (frontmatter: description, alwaysApply)
  partials/*.tmpl           shared {{define}} blocks, parsed into every .tmpl render
global/skills/<name>/...    more skill sources (same namespace)
agents/<role>/skills/<name>/...   more skill sources (same namespace; the role's AGENT.md is never touched)
projects/<name>/{skills,agents}/**   verbatim per-project files
```

- **One namespace for skills**: the `templates/skills` overlay, the embedded examples, `global/skills` and
  `agents/*/skills`. A name defined twice among the config sources (overlay, global, roles) is an error:
  `skill "x" defined twice: <path> and <path>`. The embedded examples are shadowed by any config source of the same
  name: a global or role source replaces the whole example skill, while the `templates/` overlay merges with it by
  path.
- **Rendering rule.** Files ending `.tmpl` render through `text/template` (suffix dropped, `missingkey=error`,
  partials parsed in). Every other file is copied byte for byte, so a script containing `{{ }}` stays safe. Skills are
  walked recursively. This applies to all skill sources, including `global/` and role skills.
- Agents and rules are single files from the template set. A name defined as both `x.md` and `x.md.tmpl` is an error.
- `<config>/projects/<name>/{skills,agents}/**` are the exception to the rendering rule: copied as they are, with no
  template pass, into each of the project's harnesses (see Decisions and refinements).
- A leftover `templates/cursor/` directory (the 0.2 layout for Cursor rules) is reported by `validate`.

## Template data and funcs

Every `.tmpl` renders against:

```
.Name        project or home name
.Kind        "project" | "home"
.Harness     claude | codex | cursor — the harness this copy is rendered for
.RepoRoot    a project's absolute repo root; "" for a home
.TargetRoot  the directory the file lands under: relative to the repo in a project (.claude, .agents,
             .cursor), the absolute harness dir in a home
.Vars        the merged vars (a map)
```

`.Harness` of the project's shared `.claude` tree (Claude and Cursor share it, so it is rendered once) is `claude`
when `claude` is among the project's harnesses and `cursor` otherwise; the Codex copy is `codex`; Cursor rules are
`cursor`. `.TargetRoot` is what keeps an in-repo path correct for the tree the file actually lands in.

Funcs: `dash` (spaces to hyphens), `join` (`join list ", "`; accepts YAML lists), `default` (`default "x" v`, for
nil, `""`, `0`, `false`, empty list or map), `has` (`has .Vars "a.b"`: a dotted key path exists; an existing key counts
even when its value is empty), plus the `text/template` built-ins. Optional values use `has` or `index`; a typo in a
required `.Vars.x.y` is a render error naming the file. `{{default "x" .Vars.a}}` with `a` absent is still an error,
because the key lookup fails before `default` runs. The full reference is [`template-reference.md`](template-reference.md).

## Vars merge

A project or home gets the merge of three layers, lowest first: the manifest's top-level `vars`, `defaults.vars`,
and the entry's own `vars`. Maps merge recursively key by key; scalars and lists replace; a `null` value deletes the
key (so `e2e: null` removes an inherited `e2e`). The result is a deep copy.

## Manifest

```yaml
engine: ">=0.3.0"
vars: {}                         # config-wide, the lowest layer
defaults:
  workspace: /path               # a relative repoRoot resolves against it
  harnesses: [claude, codex, cursor]
  vars: {}                       # the middle layer
projects:
  <name>:
    repoRoot: path
    harnesses: [claude, codex, cursor]   # default: defaults.harnesses
    skills: [..]
    agents: [..]
    rules: [..]
    bundles: [..]
    disabled: [..]               # bundles declared but switched off (pruned)
    vars: {}                     # the top layer
homes:
  <name>:
    harness: claude|codex|cursor
    dir: ~/.claude               # the harness user dir; ~ is expanded, the result must be absolute
    skills: [a, b]               # omitted = every global skill (see Decisions and refinements)
    skip: [q]                    # names this home does not take (skills, agents, rules)
    agents: [..]
    rules: [..]
    bundles: [..]
    vars: {}
```

A project or home installs the union of its own `skills`, `agents` and `rules` lists and the contents of its active
bundles (a project's bundles minus `disabled`). A project's `repoRoot` is required. A home's name must not collide with
a project's, because `install <name>` would be ambiguous. Unknown keys other than the removed ones below are ignored.

## Bundles

`templates/bundles.yaml` is a map from bundle name to its entry. The registry is data, never code, and nothing in the
engine knows a bundle by name. A config's own `bundles.yaml` **shadows** the embedded one: it is not merged, and the
config defines exactly the bundles it lists. The file is decoded strictly: an unknown field is an error.

| Field | Meaning |
|---|---|
| `skills`, `agents`, `rules` | names from the source namespace |
| `requires` | var paths (dotted, e.g. `[branch, gates, tracker.kind]`) that `validate` checks against the merged vars of every project and home that lists the bundle, disabled ones included |
| `sessionContext` | a file under `.claude/skills/` printed at every session start; at most one active bundle per project may set it |

`requires` tells `validate` which vars a bundle's templates read, so an entry is held only to what its bundles need;
the check is existence, as `has` does it. The shipped example is `ship`
([`examples/bundles.yaml`](../examples/bundles.yaml)).

## Harness adapters

An adapter is a small table value in `internal/engine/adapters.go`. Render, write, track and prune only ask it; adding
a harness means adding one adapter value (plus tests and this table), and the core does not change.

| item | claude | codex | cursor |
|---|---|---|---|
| project skill | `.claude/skills/<n>/` | `.agents/skills/<n>/` | shares `.claude/skills` (Cursor reads it) |
| project agent | `.claude/agents/<n>.md` | `.codex/agents/<n>.toml` and a readable copy `.agents/agents/<n>.md` | shares `.claude/agents` |
| project rule | skipped | skipped | `.cursor/rules/<n>.mdc` |
| home skill | `<dir>/skills/<n>/` | `<dir>/skills/<n>/` | `<dir>/skills/<n>/` |
| home agent | `<dir>/agents/<n>.md` | `<dir>/agents/<n>.toml` | `<dir>/agents/<n>.md` |
| home rule | skipped | skipped | `<dir>/rules/<n>.mdc` |
| session hook | `.claude/settings.json` (project) | none | via `.claude/settings.json` |

- Cursor reads `.claude/skills` and `.claude/agents` natively; a parallel `.cursor` copy would double-load every
  skill (Cursor does not dedupe same-named skills). It therefore contributes only its rules. The harness that reaches a
  shared destination first renders it (Claude before Cursor); the other skips it.
- Each adapter also declares its **roots**, the project directories it owns: claude `.claude`; codex `.agents` and
  `.codex`; cursor `.cursor` and `.claude`. Roots drive prune safety and `--target` ownership.
- Codex agent conversion: the Markdown agent becomes TOML with `name`, `description` and `developer_instructions` (the
  body). `model: sonnet` pins a Codex model and a reasoning effort. An agent whose `tools` contain neither `Edit` nor
  `Write` gets `sandbox_mode = "read-only"`. A Markdown rule becomes `.mdc` for Cursor.
- A kind a harness does not support in a scope is skipped silently.

## Write, track, prune

- **Writes are sha256-idempotent**: status per file is `install`, `update`, `skip` or `remove`; `--force` rewrites
  anyway. A file whose content starts with `#!` is written `0755`.
- **Projects** track what they wrote in `.claude/ao-manifest.json`, a JSON array of repo-relative paths. Only tracked
  paths under `.claude`, `.agents`, `.codex` or `.cursor` are ever pruned, never a directory scan, so a file trafaret
  did not write is never touched. `settings.json` is managed separately by the hook logic.
- **Homes** track per written directory: one `.ao-manifest.json` in each of `<dir>/skills/`, `<dir>/agents/` and
  `<dir>/rules/`, with paths relative to that directory. A directory that never held anything gets no track file. The
  harness's own files in `<dir>` (its other skills, its settings) are never touched.
- The track file names keep their historical `ao-` prefix: renaming them would orphan existing installs.
- **`--target claude|cursor|codex|both|all`** overrides a project's harnesses for the run and never prunes the tracked
  files of other harnesses; for homes it selects only the homes whose harness is among the targets.
- **`--what a,b`** installs only the named bundles' contents (the entry's own `skills`, `agents` and `rules` lists are
  excluded), with no pruning and no hook management, for projects and homes alike.
- **Frontmatter lint** runs on every emitted `.md` and `.mdc`: it must parse as YAML, `SKILL.md` needs a non-empty
  `name` and `description`, and no unquoted value may contain ` #`.
- Two sources that produce the same path with different bytes are an error.

## Session hook

A bundle with `sessionContext` ships a per-project `SessionStart` hook. trafaret owns exactly one entry in the
project's `.claude/settings.json` (matcher `startup|clear|compact`, command `cat` of the file under
`.claude/skills/`), merges it additively, preserves every other key, updates it when another bundle's context takes
over, and removes it when the bundle is disabled (deleting `settings.json` if nothing else was in it). It is managed
only on full installs of a project that has `claude` or `cursor` among its harnesses, and never in a home. A
`--target` install that names no harness reading that file (`codex` alone) leaves it as it is: the hook belongs to
another harness. A `settings.json` that is not valid JSON is refused rather than overwritten.

## Engine gate

`engine: ">=0.3.0"` in the manifest makes every command that reads it exit with a clear error on an older binary. The
constraint is an exact version (`"0.3.1"`) or comma-separated comparisons (`">=0.3.0, <1.0.0"`). The version is stamped
at build time; an unstamped `dev` build always passes; an absent field means no check; a constraint that cannot be
parsed is an error, so a typo cannot silently disable the gate. A 0.3 manifest should always pin `>=0.3.0`: an older
binary would not understand `vars`.

## validate

`trafaret validate` checks bundle references; that every skill, agent and rule named by a bundle or an entry exists;
harness names; `requires` var paths; `sessionContext` uniqueness; empty entries; a home name colliding with a project;
namespace duplicates; a legacy `templates/cursor/`; and then renders **every** declared item of every project and
home, so a template typo, a missing var or a lint error surfaces before install. It exits 1 with one line per problem.

## Removed (breaking)

Typed project fields `tickets`, `run`, `gates`, `docs`, `stack`, `tools`, `vcs`, `e2e` and every token derived from
them; `targets` (renamed `harnesses`); home `skillsDir` (replaced by `harness` + `dir`); `defaults.targets`,
`defaults.tickets` and `defaults.tools`; bundles' `cursorRules` (renamed `rules`); `requires: [tickets|db]` (now var
paths). The removed manifest fields are rejected loudly, not ignored:

```
projects.acme: "tickets" was removed in trafaret 0.3 — move it under vars: (see docs/spec.md, Migration)
```

## Migration of a v0.2 config

1. Move the removed project blocks under `vars:`, keys unchanged, lower-camel as written in YAML. Blocks that sat
   under `defaults:` (`tickets`, `tools`) move under `defaults.vars:`.
2. Rewrite template tokens: `.Tickets.Space` becomes `.Vars.tickets.space`. Each path segment becomes the YAML key as
   written: `.Tickets.States.InProgress` becomes `.Vars.tickets.states.inProgress`, `.VCS.DefaultBranch` becomes
   `.Vars.vcs.defaultBranch`. `.Name`, `.RepoRoot` and `.TargetRoot` are unchanged.
3. Move derived values (what the Work-mode filters or the run-profile expansion used to compute) into
   `templates/partials/` defines and call them with `{{template "name" .}}`.
4. `targets` becomes `harnesses`; home `skillsDir: ~/.claude/skills` becomes `harness: claude` plus `dir: ~/.claude`.
5. In `bundles.yaml`, `cursorRules` becomes `rules`, with the sources moved from
   `templates/cursor/rules/x.mdc.tmpl` to `templates/rules/x.md.tmpl`; `requires: [tickets]` or `[db]` becomes a list
   of var paths such as `[tickets.space, tickets.config]`.
6. Set `engine: ">=0.3.0"`.
7. Global and role skills now go through the rendering rule too: a file there ending in `.tmpl` was copied verbatim
   by 0.2 and is rendered now, losing its suffix. Rename such a file or escape its braces.

Output paths and track files are identical to v0.2, so after migrating, `trafaret diff --all` should show every
unchanged file as `skip`; anything else is a difference to read before installing.

Before and after, for one project block:

```yaml
# v0.2
defaults:
  targets: [claude, cursor, codex]
projects:
  acme-app:
    repoRoot: acme-app
    bundles: [work]
    tickets:
      cli: acme-tickets
      space: "Acme development"
      boards: { tasks: Tasks, bugs: Bugs }
```

```yaml
# v0.3
engine: ">=0.3.0"
defaults:
  harnesses: [claude, cursor, codex]
projects:
  acme-app:
    repoRoot: acme-app
    bundles: [work]
    vars:
      tickets:
        cli: acme-tickets
        space: "Acme development"
        boards: { tasks: Tasks, bugs: Bugs }
```

In a template, the token becomes a var read, and a computed value becomes a partial:

```
{{.Tickets.Space}}          →  {{.Vars.tickets.space}}
{{.Tickets.TasksDB}}        →  {{template "tasksDB" .}}
```

with the computed value defined once in `templates/partials/tickets.tmpl`:

```
{{- define "tasksDB" -}}{{.Vars.tickets.space}}/{{.Vars.tickets.boards.tasks}}{{- end -}}
```

## The generalization principle

When a hand-written skill is turned into a template, most of it is generic engineering stance; the coupling
concentrates in a few substitutable surfaces: the ticket protocol, the docs taxonomy, the per-package gate matrix and
the stack conventions. All of them are `vars` now, read through `.Vars` and, when they need logic, through partials.
Stack-specific coding rules (framework idioms, migration ordering, env-prefix examples) stay out of the templates:
replace them with a reference to the project's own policy docs, named by a var, and keep only the transferable stance
in the template.

Anything private to a deployment — hosts, workspace URLs, account names, product names of in-house tools — belongs in a
manifest var or in the config directory, never in an embedded template. This repository is public, and a test enforces
it.

## Decisions and refinements

- **A home with `skills:` omitted takes every global skill**: everything under `<config>/global/skills` and
  `<config>/agents/*/skills`, exactly as 0.2's "all". Skills from `templates/skills` and the embedded examples reach a
  home only when named in its `skills:` or through a bundle. An explicit `skills: []` means none. The first design
  said "every skill in the namespace"; the narrower meaning keeps a migrated config byte-identical.
- **`null` deletes a var key.** It is how a project removes something it inherited (`e2e: null`), and `has` is then
  false for it.
- **`default` versus absent keys.** `default` returns its fallback for a value that exists but is empty; it does not
  rescue a key that is missing, because `missingkey=error` fires first. Optional values use `has` or `index`. An
  existing key counts as present for `has` even when its value is `""` or `0`.
- **Files starting with `#!` are written `0755`**, rendered `.sh.tmpl` files included, so a rendered script is
  runnable.
- **`projects/<name>/` stays verbatim.** The spec was silent about the 0.2 per-project source directory; it is kept:
  copied as is (no template pass) into each of the project's harnesses, and for Codex a Markdown agent becomes TOML
  only, with no readable `.agents/agents` copy.
- **Removed fields are rejected loudly**, with the replacement named in the message, rather than ignored: a silently
  ignored `tickets:` would surface later as a confusing "map has no entry for key" inside a template.
- **`bundles.yaml` is strict.** Unknown fields are an error, and `cursorRules` gets a rename hint, for the same reason.
- **The shared Claude/Cursor tree renders once**, as `claude` when `claude` is among the harnesses. A Cursor-only
  conditional in a skill fires only for a Cursor-only project; Cursor-specific prose belongs in a rule.
- **`list`** prints `PROJECT HARNESSES BUNDLES SKILLS` and `HOME HARNESS DIR SKILLS`; `list --json` prints
  `{"projects": {...}, "homes": {...}}`.

## Out of scope

A ticket-system registry, memory sync, settings and statusline bundling, and skill wrappers for individual CLIs beyond
what a config's `global/skills/` carries. The engine also ships no process library of its own: skills, agents and
bundles are the user's to write.
