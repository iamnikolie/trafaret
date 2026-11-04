# Template reference

How a source file becomes a rendered file: which files are templates, what a template can read, the
functions it can call, how `vars` are merged, how to handle optional values, and where every kind of
item lands in each harness.

Everything quoted here comes from the embedded example set ([`examples/`](../examples/)) and the committed
output of the demo config ([`examples/demo/out/`](../examples/demo/out/)). The README walks through that
demo in [How templating works](../README.md#how-templating-works); this file is the reference behind it.

## Which files are templates

| Source | Treatment |
|---|---|
| a file whose name ends in `.tmpl` | rendered with Go `text/template`; the `.tmpl` suffix is dropped (`SKILL.md.tmpl` becomes `SKILL.md`, `gates.sh.tmpl` becomes `gates.sh`) |
| any other file | copied byte for byte |
| `<config>/projects/<name>/skills\|agents/**` | copied byte for byte into that project, whatever the name; no template pass at all |

- A skill is a directory and is walked recursively: every `.tmpl` file in it renders, every other file is
  copied. This holds for every skill source: the `templates/skills` overlay, the embedded examples,
  `global/skills` and `agents/<role>/skills`.
- Agents and rules are single files: `templates/agents/<name>.md.tmpl` or `.md`, `templates/rules/<name>.md.tmpl`
  or `.md`. Defining the same name as both `x.md` and `x.md.tmpl` is an error. To replace an embedded agent or
  rule from your overlay, keep its exact file name, suffix included.
- A script that contains `{{ }}` of its own stays safe as long as it has no `.tmpl` suffix: the demo's
  [`my-reviews.sh`](../examples/skills/ship/scripts/my-reviews.sh) holds a `gh` template and reaches the repo
  untouched (see [its output](../examples/demo/out/acme-api/.claude/skills/ship/scripts/my-reviews.sh)).
  Inside a `.tmpl` file, write a literal brace pair as `{{"{{"}}` and `{{"}}"}}`.
- A written file whose content starts with `#!` gets mode `0755`; everything else `0644`. An existing file with
  identical content is not touched.
- Rendering uses `missingkey=error`: reading a key that is not in a map is a render error that names the file
  and the line, never an empty string. `trafaret validate` renders every declared item of every project and
  home, so a typo surfaces there.

## Template data

Every `.tmpl` file renders against one value, the dot (`.`):

| Field | Type | Value |
|---|---|---|
| `.Name` | string | the project or home name: the key in `projects:` or `homes:` |
| `.Kind` | string | `project` or `home` |
| `.Harness` | string | `claude`, `codex` or `cursor`: the harness this copy is rendered for |
| `.RepoRoot` | string | a project's absolute repo root (`defaults.workspace` + `repoRoot`, `~` expanded); `""` in a home |
| `.TargetRoot` | string | the directory the file lands under: relative to the repo in a project (`.claude`, `.agents`, `.cursor`), the absolute harness dir in a home |
| `.Vars` | map | the merged vars of this project or home (next section); nested maps are `map[string]any`, lists `[]any`, numbers Go `int` or `float64`, plus strings and bools |

`.Harness` and `.TargetRoot` depend on the placement, because one source can be rendered more than once. Exactly:

| Where the file lands | `.Harness` | `.TargetRoot` |
|---|---|---|
| project skill or agent in the shared Claude/Cursor tree (`.claude/skills`, `.claude/agents`) | `claude`; `cursor` only when the project's harnesses contain `cursor` but not `claude` | `.claude` |
| project skill (`.agents/skills`) | `codex` | `.agents` |
| project agent for Codex (`.agents/agents/<n>.md` and `.codex/agents/<n>.toml`, both from one render) | `codex` | `.agents` |
| project rule (`.cursor/rules`) | `cursor` | `.cursor` |
| any item in a home | the home's `harness` | the home's absolute `dir` (for example `/home/me/.claude`) |

- Claude and Cursor read the same project tree (`.claude/skills`, `.claude/agents`), so it is rendered once, not
  twice. With both harnesses in the project that copy is the `claude` one. A Cursor-only conditional in a skill
  therefore fires only for a project whose harnesses are `[cursor]`; Cursor-only prose belongs in a rule.
- A project's `.claude` tree for Claude and the `.agents` tree for Codex are separate renders of the same source,
  which is why `.TargetRoot` exists: use it for every in-repo path to a skill or agent file instead of writing
  `.claude/` by hand. The demo's gate line renders as ``.claude/skills/ship/scripts/gates.sh`` in
  [`.claude/skills/ship/SKILL.md`](../examples/demo/out/acme-api/.claude/skills/ship/SKILL.md) and as
  ``.agents/skills/ship/scripts/gates.sh`` in
  [`.agents/skills/ship/SKILL.md`](../examples/demo/out/acme-api/.agents/skills/ship/SKILL.md).
- In a home `.RepoRoot` is empty and `.TargetRoot` is absolute, so a home template should not build repo-relative
  paths from it.

## Functions

The engine adds four functions to `text/template`'s built-ins:

| Func | Meaning | Example |
|---|---|---|
| `dash` | spaces to hyphens (trimmed first) | `{{dash "team memory"}}` renders `team-memory` |
| `join` | `strings.Join` over a list, elements first; accepts YAML lists, and `nil` gives `""` | `{{join .Vars.labels ", "}}` renders `bug, chore` |
| `default` | `default "x" v` returns `"x"` when `v` is empty: `nil`, `""`, `0`, `false`, an empty list or map | `{{default "main" .Vars.branch}}` |
| `has` | `has map "a.b"`: the dotted key path exists. An existing key counts as present even when its value is `""` or `0`; a missing key, or a path through a non-map, is false. Works on any map | `{{if has .Vars "tracker.host"}}` |

Built-ins you will use most, with the demo's own lines:

| Built-in | Example |
|---|---|
| `len` | `runs the {{len .Vars.gates}} gates` renders `runs the 2 gates` |
| `range` | `` {{range .Vars.gates}}- **{{.name}}** — `{{.cmd}}` {{end}} ``: inside, dot is the element, so `{{.name}}` is the gate's `name` |
| `if` / `else` / `eq` | `{{if eq .Vars.tracker.kind "jira"}}…{{else}}…{{end}}` |
| `gt` `lt` `ge` `le` `ne` | `{{if gt .Vars.reviewers 1}}s{{end}}`: plural when more than one approval. Both sides need the same kind of number: a YAML `2` is an `int`, so compare it with `1`, not `1.0` |
| `index` | `{{index .Vars "e2e"}}`; `nil` when the key is missing (see Optional values) |
| `and` `or` `not` `with` `printf` | as in the standard library |

Dot changes inside `range` and `with`; inside them reach the outer data through a variable: `{{$root := .}}` before
the `range`, then `{{$root.Name}}` inside.

## Vars and the merge

`vars` are free-form YAML. The engine gives them no meaning; a template reads them as `.Vars.<key>`. A project or
home gets the merge of three layers, lowest first:

| Layer | Where |
|---|---|
| 1 | `vars:` at the top level of `manifest.yaml`: config-wide |
| 2 | `defaults.vars:`: every project and home |
| 3 | the entry's own `vars:` under `projects.<name>` or `homes.<name>` |

Rules, applied left to right:

- **Maps merge recursively**, key by key.
- **Scalars and lists replace.** A list is never appended to.
- **`null` deletes the key**, so a project can remove something it inherited.
- The result is a deep copy: no project's vars can leak into another's.

Worked example:

```yaml
vars:                                  # layer 1
  company: Acme
  labels: [bug, chore]
  tracker: { kind: github, reviewers: 1 }
defaults:
  vars:                                # layer 2
    branch: main
    e2e: make e2e
    tracker: { repo: acme/app }
projects:
  web:
    repoRoot: web
    vars:                              # layer 3
      tracker: { kind: jira, host: acme.atlassian.net }
      labels: [feature]
      e2e: null
```

The vars of `web` after the merge:

```yaml
company: Acme
branch: main
labels: [feature]                      # the list replaced layer 1's list
tracker:                               # the maps merged key by key
  kind: jira                           #   replaced by layer 3
  reviewers: 1                         #   kept from layer 1
  repo: acme/app                       #   kept from layer 2
  host: acme.atlassian.net             #   added by layer 3
                                       # e2e is gone: layer 3 set it to null
```

The demo shows the same thing with real files: `acme-api` sets only `tracker: { repo: acme/api }` and keeps
`kind: github` from `defaults.vars`, while `acme-web` replaces `branch` and `reviewers` and sets its own
`tracker` and an `e2e` var. Compare [`examples/demo/manifest.yaml`](../examples/demo/manifest.yaml) with the two
rendered [`ship/SKILL.md`](../examples/demo/out/acme-api/.claude/skills/ship/SKILL.md) files
([`acme-web`](../examples/demo/out/acme-web/.claude/skills/ship/SKILL.md)).

## Optional values

A template that reads `.Vars.x.y` requires the key. Under `missingkey=error` a missing key stops the render:

```
render examples/skills/ship/SKILL.md.tmpl: … at <.Vars.gates>: map has no entry for key "gates"
```

For a value that may be absent, use one of:

| Need | Write | Notes |
|---|---|---|
| render a block only when the key exists | `{{if has .Vars "e2e"}}…{{end}}` | the demo's e2e gate: present for `acme-web`, absent for `acme-api` |
| read a value that may be absent | `{{index .Vars "e2e"}}` | `index` returns `nil` for a missing key and does not error |
| test a possibly absent key for truthiness | `{{if index .Vars.tracker "host"}}` | works one level down the same way |
| a fallback for an absent key | `{{default "none" (index .Vars "e2e")}}` | `index` first, then `default` |
| a fallback for a present but empty value | `{{default "main" .Vars.branch}}` | `""`, `0`, `false`, empty list and map all count as empty |

The caveat: **`default` does not rescue an absent key.** `{{default "x" .Vars.a}}` with `a` missing is still a render
error, because the key lookup fails before `default` runs. Use `has` or `index` for absent keys, and `default` for
values that exist but may be empty. `{{if .Vars.e2e}}` has the same trap: with `e2e` missing it is an error, not
`false`; write `{{if has .Vars "e2e"}}`.

Two edge cases:

- `has` splits its path on dots, so it cannot address a key that itself contains a dot. A key that is not a valid
  Go identifier (`my-key`) cannot be written `.Vars.my-key`; read it as `{{index .Vars "my-key"}}`.
- A bundle's `requires` list uses the same dotted-path rule as `has`: a key that exists passes even if its value is
  empty. `requires` only checks existence; it does not check types.

## Partials

Every `*.tmpl` file under `templates/partials/` is parsed into every `.tmpl` render. Put `{{define "name"}}…{{end}}`
blocks in it and call one with `{{template "name" .}}`, passing the dot so the partial sees `.Vars`, `.Name` and the
rest. Text outside the defines is ignored, so a partial file can carry a leading comment. A define name is global
across all partial files; a file in your `templates/partials/` with the same path as an embedded one shadows it.

The embedded [`partials/tracker.tmpl`](../examples/partials/tracker.tmpl) shows the use: it turns `vars.tracker`
into an issue reference and a URL, so a skill never branches on the tracker itself.

```
{{- define "issueRef" -}}
{{- if eq .Vars.tracker.kind "jira" -}}{{.Vars.tracker.key}}-123{{- else -}}#123{{- end -}}
{{- end -}}
```

The skill calls `{{template "issueRef" .}}`, which renders `#123` for `acme-api` and `WEB-123` for `acme-web`. Derived
values that used to be computed by the engine in v0.2 (a database path built from two fields, a priority chain)
are partials now; see Migration in [`spec.md`](spec.md).

## Frontmatter conventions and the lint

The frontmatter of every emitted `.md` and `.mdc` file is linted after rendering, whether the file was rendered,
copied or converted: it must be terminated, it must parse as YAML, and no unquoted value may contain ` #` (YAML
starts a comment there and silently drops the rest of the line). A file with no frontmatter passes. The error names
the key: `frontmatter "description" contains ' #' in an unquoted value`. Keep `: ` and ` #` out of any var that is
inserted into an unquoted frontmatter value, or quote the value in the template.

| Kind | Source | Frontmatter | Notes |
|---|---|---|---|
| skill | directory `skills/<name>/` with `SKILL.md` or `SKILL.md.tmpl` | `name`, `description`: both non-empty, enforced by the lint | the directory name is what `skills:` and bundles list; keep it equal to `name`. Other files of the directory (scripts, references, a session-context file) ride along |
| agent | `agents/<name>.md[.tmpl]` | `name`, `description`, `tools` (comma list), `model` | Codex needs `name` and `description` to convert; see the table below for `tools` and `model` |
| rule | `rules/<name>.md[.tmpl]` | `description`, `alwaysApply` | becomes a Cursor `.mdc` file; Claude and Codex have no rules and skip them |

Conventions worth following:

- A project-scoped skill ends its description with a scope clause such as `Applies only to the {{.Name}} repo.`
  (the demo's `ship` does): same-named skills from different repos can load side by side in one workspace, and
  the clause is what tells them apart. A home skill is global and carries none.
- Keep agents thin: an agent file points at its skill; the skill stays the single source of every checklist.

How an agent's frontmatter becomes Codex TOML (`.codex/agents/<name>.toml`, project and home alike):

| Frontmatter | TOML |
|---|---|
| `name`, `description` | `name`, `description` (both required) |
| `model: sonnet` | pins a Codex model and `model_reasoning_effort = "medium"`; any other `model` value adds nothing |
| `tools` without `Edit` and `Write`, or no `tools` line | `sandbox_mode = "read-only"` |
| the Markdown body | `developer_instructions`, one quoted string |

See [`gatekeeper.md.tmpl`](../examples/agents/gatekeeper.md.tmpl) and its
[TOML](../examples/demo/out/acme-api/.codex/agents/gatekeeper.toml).

## Where each item lands

One source, placed by the harness adapter. Projects first, then homes; `<dir>` is the home's `dir`.

| Item | claude | codex | cursor |
|---|---|---|---|
| project skill | `.claude/skills/<n>/` | `.agents/skills/<n>/` | shares `.claude/skills` |
| project agent | `.claude/agents/<n>.md` | `.codex/agents/<n>.toml` and a readable copy `.agents/agents/<n>.md` | shares `.claude/agents` |
| project rule | skipped | skipped | `.cursor/rules/<n>.mdc` |
| home skill | `<dir>/skills/<n>/` | `<dir>/skills/<n>/` | `<dir>/skills/<n>/` |
| home agent | `<dir>/agents/<n>.md` | `<dir>/agents/<n>.toml` | `<dir>/agents/<n>.md` |
| home rule | skipped | skipped | `<dir>/rules/<n>.mdc` |
| session hook | `.claude/settings.json` | none | via `.claude/settings.json` |

Notes:

- Cursor reads `.claude/skills` and `.claude/agents` natively, so it contributes only its rules. A parallel
  `.cursor/skills` would load every skill twice. Never create one.
- A kind a harness does not support in a scope is skipped without a message: listing a rule for a project whose
  harnesses are `[claude]` renders nothing.
- A project's verbatim `projects/<name>/skills` go to wherever the harness puts skills; its `*.md` files in
  `projects/<name>/agents` go where it puts agents (a Codex agent becomes TOML only, with no readable copy).
- Two sources that produce the same path with different bytes are an error (for example a verbatim
  `projects/<name>` file that collides with a rendered one); the same bytes are fine.

## Session hook

A bundle may set `sessionContext: <skill>/<file>`, a file under `.claude/skills/` (the rendered file, so the source is
usually `<file>.tmpl`). trafaret then owns exactly one `SessionStart` entry in the project's `.claude/settings.json`
that prints the file at `startup`, `clear` and `compact`:

```
"command": "cat \"$CLAUDE_PROJECT_DIR/.claude/skills/ship/session-context.md\""
```

It is merged additively (every other key is preserved), updated when another bundle's context takes over, and removed
when the bundle is disabled. At most one active bundle per project may set `sessionContext`. It is managed only on
full installs (no `--what`) of a project with `claude` or `cursor` among its harnesses, and never in a home. The demo
shows both halves: [`session-context.md.tmpl`](../examples/skills/ship/session-context.md.tmpl) renders to
[`session-context.md`](../examples/demo/out/acme-api/.claude/skills/ship/session-context.md), and the hook is
[`.claude/settings.json`](../examples/demo/out/acme-api/.claude/settings.json).

## Recipe: writing a harness-conditional section

Use this when Claude, Codex and Cursor need different words for the same step.

1. Decide what differs. A path to a file the harness owns is `{{.TargetRoot}}`, not a conditional. A different
   mechanism (a subagent in Claude, a custom agent in Codex) is a conditional on `.Harness`.
2. Branch on the harness that is the exception and let the others share the `else`:

   ```
   {{if eq .Harness "codex" -}}
   Want a second pair of eyes? Run the `gatekeeper` custom agent (`.codex/agents/gatekeeper.toml`).
   {{- else -}}
   Want a second pair of eyes? Dispatch the `gatekeeper` subagent — it re-runs the gates read-only.
   {{- end}}
   ```

   That is lines 20 to 24 of [`ship/SKILL.md.tmpl`](../examples/skills/ship/SKILL.md.tmpl). The `-` trim markers keep
   the blank lines around the block from doubling. For three variants use `{{else if eq .Harness "cursor"}}`.
3. Remember the shared tree: a project with `claude` and `cursor` renders its `.claude` skills once, as `claude`.
   Branch on `cursor` only for rules, or when you know the project is Cursor-only.
4. Run `trafaret validate` (it renders every harness of every project and home), then `trafaret diff <name>` and open
   the rendered files. Compare against the demo's pair of
   [`.claude`](../examples/demo/out/acme-api/.claude/skills/ship/SKILL.md) and
   [`.agents`](../examples/demo/out/acme-api/.agents/skills/ship/SKILL.md) outputs to see what a conditional produced.

## Render errors

| Message | Meaning |
|---|---|
| `render <file>: … map has no entry for key "x"` | a template reads `.Vars.x` and no layer sets it: set the var, or make the read optional (`has` / `index`) |
| `render <file>: … can't evaluate field X in type …` | a field that does not exist on the template data; the fields are listed at the top of this file |
| `parse <file>: … function "f" not defined` | a func that is not `dash`, `join`, `default`, `has` or a built-in |
| `frontmatter … contains ' #' in an unquoted value` | quote the value or drop the `#` |
| `frontmatter missing non-empty "description"` | `SKILL.md` needs both `name` and `description` |
| `<path> is produced twice with different content` | two sources write the same destination; rename one |
| `projects.<p>: "tickets" was removed in trafaret 0.3 — move it under vars:` | a v0.2 manifest field; see Migration in [`spec.md`](spec.md) |
