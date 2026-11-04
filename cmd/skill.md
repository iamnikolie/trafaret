# trafaret — write a skill, agent or rule once, render it into every harness (agent reference)

`trafaret` renders a library of agent skills, subagents and rules — Go templates
plus free `vars` from a manifest — into Claude Code, Codex and Cursor, for
per-repo **projects** and for user-level **homes** (`~/.claude`, `~/.codex`,
`~/.cursor`) through one pipeline. The engine knows harness layouts and nothing
else: tickets, gates, docs and stack are just values under `vars:`. Writes are
sha256-idempotent: an unchanged file is not touched. What it writes is a build
artifact; the source is the template or the manifest, never the rendered tree.

## Shape of an invocation

```
trafaret [--config DIR] <command> [project|home] [flags]
```

Flags work before or after the command: `trafaret --config . validate` and
`trafaret validate --config .` are the same call.

The **config directory** holds everything that is the user's:

```
<config>/
  manifest.yaml                 required: vars, defaults, projects, homes
  templates/                    optional overlay on the embedded examples
    bundles.yaml                  bundle registry (shadows the embedded one)
    skills/<name>/                SKILL.md[.tmpl] + any other files
    agents/<name>.md[.tmpl]       frontmatter: name, description, tools, model
    rules/<name>.md[.tmpl]        frontmatter: description, alwaysApply
    partials/*.tmpl               {{define}} blocks shared by every .tmpl
  global/skills/<name>/         more skills (same namespace)
  agents/<role>/skills/<name>/  more skills; <role>/AGENT.md is never touched
  projects/<name>/{skills,agents}/   verbatim per-project files, no template pass
```

It is found, in order, by `--manifest PATH` (its directory), `--config DIR`,
`$TRAFARET_CONFIG`, then the nearest parent of the working directory that has a
`manifest.yaml`. `trafaret root` prints the one it resolved. `version`, `skill`
and `help` need no config.

## Commands

| Command | Purpose |
|---|---|
| `list [--json]` | projects (name, harnesses, bundles, skills) and homes (name, harness, dir, skills) |
| `validate` | check the manifest and render every declared item of every project and home; exit 1 when anything is wrong |
| `diff <project\|home>` | dry-run: per-file `install` / `update` / `skip` / `remove` |
| `install <project\|home>` | render and write; prunes what a later render no longer produces |
| `install --all` | every home, then every project in the manifest |
| `root` | print the resolved config directory |
| `serve [--addr host:port]` | local live manual, re-rendered from the config on every refresh (default `127.0.0.1:4477`) |
| `skill` | this document |
| `version` | version, to stdout |

A name is a home if the manifest has a home of that name, otherwise a project.

## Flags

| Flag | Meaning |
|---|---|
| `--config DIR` | config directory; or `$TRAFARET_CONFIG` |
| `--manifest PATH` | manifest file; its directory becomes the config directory |
| `--templates PATH` | templates overlay directory instead of `<config>/templates` (must exist) |
| `--all` | every project and home (install, diff) |
| `--target claude\|cursor\|codex\|both\|all` | override a project's harnesses for this run; a home is installed only when its harness is among them |
| `--what a,b` | install only these bundles' contents (a subset of the entry's); no pruning, no hook management |
| `--force` | rewrite even sha256-equal files |
| `--dry-run` | render and report, write nothing |
| `--verbose`, `-v` | also list `skip` lines |
| `--json` | machine-readable output (list, install, diff) |
| `--addr host:port` | serve: listen address; a bare port binds localhost |
| `--version`, `-V` | print the version, same as the `version` command |

## What gets written

A project installs the union of its own `skills:`, `agents:` and `rules:` lists
and the contents of its active bundles (`bundles:` minus `disabled:`), once per
harness in `harnesses:` (default `defaults.harnesses`). A home does the same for
its single harness. Where each item lands is the adapter table:

| Item | `claude` | `codex` | `cursor` |
|---|---|---|---|
| project skill | `.claude/skills/<n>/` | `.agents/skills/<n>/` | shares `.claude/skills` |
| project agent | `.claude/agents/<n>.md` | `.codex/agents/<n>.toml` + readable `.agents/agents/<n>.md` | shares `.claude/agents` |
| project rule | skipped | skipped | `.cursor/rules/<n>.mdc` |
| home skill | `<dir>/skills/<n>/` | `<dir>/skills/<n>/` | `<dir>/skills/<n>/` |
| home agent | `<dir>/agents/<n>.md` | `<dir>/agents/<n>.toml` | `<dir>/agents/<n>.md` |
| home rule | skipped | skipped | `<dir>/rules/<n>.mdc` |
| session hook | `.claude/settings.json` | none | via `.claude/settings.json` |

Cursor reads `.claude/skills` and `.claude/agents` natively, so the shared tree is
rendered once (Claude first) and Cursor adds only its rules. Never create
`.cursor/skills` or `.cursor/agents`: every skill would load twice. A kind a
harness does not support in a scope is skipped silently. A Codex agent is
converted to TOML: `model: sonnet` pins a Codex model, and an agent whose `tools`
has neither `Edit` nor `Write` (or no `tools` line) gets `sandbox_mode = "read-only"`.

`projects/<name>/skills` and `projects/<name>/agents/*.md` are copied verbatim into
each of the project's harnesses (a Codex agent becomes TOML only, no readable
copy). A written file starting with `#!` gets mode 0755.

Tracking, so only what trafaret wrote is ever pruned:

- project: `.claude/ao-manifest.json`, a JSON array of repo-relative paths; only
  tracked paths under `.claude`, `.agents`, `.codex`, `.cursor` are pruned;
- home: one `.ao-manifest.json` in each written directory (`<dir>/skills/`,
  `<dir>/agents/`, `<dir>/rules/`), paths relative to it; the harness's own files
  in `<dir>` are never touched.

`--target` for a project never prunes another harness's tracked files. Two sources
that produce the same path with different bytes are an error.

## manifest.yaml

```yaml
engine: ">=0.3.0"              # refuse older binaries
vars: { company: Acme }        # config-wide, lowest layer
defaults:
  workspace: /work             # relative repoRoot values resolve against it
  harnesses: [claude, codex, cursor]
  vars: { branch: main }       # middle layer: every project and home
projects:
  myproject:
    repoRoot: myproject
    harnesses: [claude, codex] # default: defaults.harnesses
    skills: []                 # names from the skill namespace
    agents: []                 # templates/agents/<name>
    rules: []                  # templates/rules/<name>
    bundles: [ship]            # names from templates/bundles.yaml
    disabled: []               # declared but switched off; the next install prunes them
    vars:                      # top layer
      lang: Go
homes:
  me:
    harness: claude            # claude | codex | cursor
    dir: ~/.claude             # ~ expanded; must be absolute after that
    skills: [standup]          # omitted = every GLOBAL skill; [] = none
    skip: [codex-only-skill]   # names this home does not take
    agents: []
    rules: []
    bundles: []
    vars: {}
```

Removed in 0.3 and rejected with a migration hint: project `tickets` `run` `gates`
`docs` `stack` `tools` `vcs` `e2e` `targets`; `defaults.targets` `defaults.tickets`
`defaults.tools`; home `skillsDir`. Move them under `vars:`; `targets` is now
`harnesses`; `skillsDir: ~/.claude/skills` is `harness: claude` + `dir: ~/.claude`.

**Vars merge**: `vars` < `defaults.vars` < the entry's `vars`. Maps merge
recursively key by key; scalars and lists replace; a `null` value deletes the key
(`e2e: null` removes an inherited `e2e`). The result is a deep copy per entry.

## Templates

Files ending `.tmpl` render through Go `text/template` (the suffix is dropped,
`missingkey=error`, every partial parsed in). Every other file is copied byte for
byte, so a `.sh` containing `{{ }}` is safe. Skills are walked recursively and this
holds for every skill source, `global/` and role skills included. Only
`projects/<name>/` is never rendered.

| Field | Value |
|---|---|
| `.Name` | the project or home name |
| `.Kind` | `project` or `home` |
| `.Harness` | `claude`, `codex` or `cursor`: the harness this copy is rendered for. The shared project `.claude` tree is `claude` when claude is a harness, else `cursor` |
| `.RepoRoot` | a project's absolute repo root; `""` in a home |
| `.TargetRoot` | where the file lands: `.claude` / `.agents` / `.cursor` in a project, the absolute harness dir in a home. Use it for any in-repo path to a skill or agent file |
| `.Vars` | the merged vars: read `.Vars.tracker.kind` |

Funcs, besides the `text/template` built-ins (`len`, `eq`, `gt`, `range`, `index`, …):

| Func | Meaning |
|---|---|
| `dash` | spaces to hyphens: `{{dash "team memory"}}` |
| `join` | `{{join .Vars.labels ", "}}`, elements first, accepts YAML lists |
| `default` | `{{default "main" .Vars.branch}}`: the fallback for nil, `""`, `0`, `false`, empty list or map |
| `has` | `{{if has .Vars "tracker.host"}}`: the dotted path exists (present even when its value is empty) |

Optional values: a missing key is a render error, so guard it with
`{{if has .Vars "e2e"}}` or read it with `{{index .Vars "e2e"}}` (nil when absent;
`{{default "x" (index .Vars "e2e")}}`). `{{default "x" .Vars.a}}` with `a` absent is
still an error: `default` helps for present-but-empty values, `index` for absent ones.

Partials: every `templates/partials/*.tmpl` holds `{{define "name"}}…{{end}}`
blocks; call one with `{{template "name" .}}`. Put computed values there.

Harness-conditional prose:

```
{{if eq .Harness "codex" -}}
Run the `gatekeeper` custom agent.
{{- else -}}
Dispatch the `gatekeeper` subagent.
{{- end}}
```

## The overlay rule

The binary embeds a small set of **example** templates (`bundles.yaml`, `skills/`,
`agents/`, `rules/`, `partials/`). The config's `templates/` directory is layered
on top:

- a file at the same path replaces the embedded one; new files are added;
  directories are merged (an overlay skill can replace one file of an example);
- `templates/bundles.yaml` **replaces** the embedded registry, it is not merged;
- an agent or rule replaces an embedded one only under the exact same file name
  (`x.md.tmpl` vs `x.md` is a duplicate error);
- skills share one namespace across the overlay, `global/skills`, `agents/*/skills`
  and the examples: a name defined twice among the config sources is an error; a
  config source shadows an embedded example of the same name.

## bundles.yaml

A bundle is a named set of items; a project or home opts in by name.

```yaml
ship:
  skills: [ship]                          # names from the skill namespace
  agents: [gatekeeper]                    # templates/agents/<name>.md[.tmpl]
  rules: [repo-stance]                    # templates/rules/<name>.md[.tmpl]
  requires: [branch, gates, tracker.kind] # var paths `validate` checks
  sessionContext: ship/session-context.md
```

All fields are optional; unknown fields are an error (strict). `requires` is
checked by `validate` against the merged vars of every project and home that
lists the bundle, disabled ones included; existence only. `sessionContext` is a
file under `.claude/skills/` that a SessionStart hook in the project's
`.claude/settings.json` prints at startup, clear and compact; trafaret owns
exactly one hook entry, merged additively, removed with the bundle, managed only on
full installs of a project with claude or cursor among its harnesses, never in a
home. At most one active bundle per project may set it.

## Rendering rules

- Frontmatter of every emitted `.md` / `.mdc` is linted: valid YAML, a non-empty
  `name` and `description` in `SKILL.md`, and no unquoted value containing ` #`
  (YAML silently drops the rest of the line).
- Project-scoped skill descriptions should end with `Applies only to the {{.Name}}
  repo.`: same-named skills load side by side in multi-root workspaces.
- Rendering is byte-stable. After any template or manifest edit run `diff` and
  read the `update` lines before installing.
- Never hand-edit the rendered trees; edit the template or the manifest.

## Recipes

```bash
# where is the config, and is it sound
trafaret root
trafaret validate

# what would change, then change it
trafaret diff myproject
trafaret install myproject
trafaret install --all

# one harness, or one bundle, without pruning the rest
trafaret install myproject --target claude
trafaret install myproject --what ship

# a user-level home
trafaret diff me
trafaret install me

# feed a script
trafaret list --json
trafaret install --all --dry-run --json

# try the embedded demo, read-only
trafaret --config examples/demo validate
trafaret --config examples/demo install acme-api --dry-run
```

## Failure modes worth recognising

Errors print as `trafaret: <message>` on stderr and exit 1; an unknown command
prints the usage screen and exits 2.

| Message | Meaning |
|---|---|
| `no manifest.yaml found in <dir> or any parent directory; pass --config DIR or set $TRAFARET_CONFIG` | nothing resolved a config directory |
| `--config <dir>: not a directory` / `no manifest.yaml in it` | the flag points at the wrong place |
| `--manifest <path>: no such file` | explicit manifest missing |
| `--templates <dir>: not a directory` | the overlay path must exist (an empty dir disables the overlay) |
| `manifest requires trafaret <constraint>, this binary is <v>` | the manifest's `engine:` is newer than the binary |
| `projects.<p>: "tickets" was removed in trafaret 0.3 — move it under vars:` | a v0.2 field; also `targets`, `skillsDir`, `defaults.tools`; see docs/spec.md, Migration |
| `install: name a project or home, or pass --all` | nothing selected |
| `no such project or home: <name>` | typo, or the name is not in the manifest |
| `invalid --target (claude\|codex\|cursor\|both\|all): <x>` | unknown target |
| `unknown harness "<x>" (want claude\|codex\|cursor)` | bad `harnesses:` entry |
| `harness "<x>" is not one of …` / `dir must be an absolute or ~/ path` | a home needs `harness:` and an absolute `dir:` |
| `name collides with a project` | a home and a project share a name |
| `bundle "<x>" is not in templates/bundles.yaml` | a config's own registry shadows the embedded one; add the bundle there |
| `bundle <b> requires var "<path>", which is not set` | set the var in `vars:`, `defaults.vars` or the entry's `vars:` |
| `templates/bundles.yaml: … field cursorRules not found` | 0.2 name; use `rules:` and `templates/rules/` |
| `templates/cursor/ is the 0.2 layout` | move Cursor rules to `templates/rules/<n>.md[.tmpl]` |
| `skill "<x>" defined twice: <path> and <path>` | one skill name in two config sources |
| `no skill\|agent\|rule "<x>" (looked in …)` | a name no source defines |
| `render error: … map has no entry for key "<x>"` | a template reads a var that no layer sets: set it, or guard it with `has` / `index` |
| `render error: … can't evaluate field X …` | a field not in the template data (`.Name .Kind .Harness .RepoRoot .TargetRoot .Vars`) |
| `<path> is produced twice with different content` | two sources write one destination |
| `frontmatter … contains ' #' in an unquoted value` | quote the value or drop the `#` |
| `bundles "<a>" and "<b>" both set sessionContext` | at most one active bundle per project |
| `<settings.json>: not valid JSON, refusing to manage the session hook` | fix the file; trafaret will not overwrite it |
| `N problem(s)` after `validate` output | each line above it names a project or `home:<name>` and the problem |
