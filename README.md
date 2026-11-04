# trafaret

[![CI](https://github.com/iamnikolie/trafaret/actions/workflows/ci.yml/badge.svg)](https://github.com/iamnikolie/trafaret/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/iamnikolie/trafaret.svg)](https://pkg.go.dev/github.com/iamnikolie/trafaret)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Write a skill, a subagent or a rule once; render it, with your own values inserted, into every harness — **Claude Code**,
**Codex** and **Cursor** — for user-level homes (`~/.claude`, `~/.codex`, `~/.cursor`) and for per-repo projects,
through one pipeline.

trafaret is a generic engine. You write the templates (Go `text/template`) and group them into **bundles**; the values
they insert live in the `vars:` of a `manifest.yaml`: free-form YAML that the engine gives no meaning, so a ticket
board, a gate list, a docs layout or a stack convention is whatever your skills need it to be. `trafaret install <name>`
renders what that project or home asked for into each harness's native locations and format: a Markdown agent for
Claude, TOML for Codex, an `.mdc` rule for Cursor. Writes are sha256-idempotent (an unchanged file is skipped), and
only files trafaret wrote are ever pruned.

The engine knows harness layouts and nothing else. The binary embeds a few **examples** that show the format, and
`examples/demo` is a runnable config with its rendered output committed (see [How templating works](#how-templating-works)).
Your real templates and bundles live in your own **config directory**.

## Install

**Homebrew:**

```bash
brew trust iamnikolie/tap   # Homebrew 6 refuses untrusted third-party taps
brew tap iamnikolie/tap
brew install iamnikolie/tap/trafaret
```

**Prebuilt binary** — from [Releases](https://github.com/iamnikolie/trafaret/releases) (linux, darwin, windows ×
amd64, arm64).

**Go:**

```bash
go install github.com/iamnikolie/trafaret@latest
```

**From a checkout:**

```bash
make install            # builds ./trafaret and symlinks ~/.local/bin/trafaret -> ./trafaret
```

The example templates are embedded in the binary — there is nothing else to install and it runs from any directory.

From a checkout, try the demo without writing anything:

```bash
trafaret --config examples/demo validate
trafaret --config examples/demo install acme-api --dry-run
```

The demo's homes are scratch directories under `/tmp/trafaret-demo/` and its projects resolve under `/work`, so it
never touches a real harness.

## The config directory

Everything that is *yours* lives in one directory, the **config directory**: the directory that holds `manifest.yaml`.

```
my-config/
  manifest.yaml          vars, defaults, projects and homes (required)
  templates/             your templates and bundles; an overlay on the embedded examples (optional)
    bundles.yaml           the bundle registry (shadows the embedded one)
    skills/<name>/         SKILL.md.tmpl + any other files of a skill
    agents/<name>.md.tmpl  subagents (frontmatter: name, description, tools, model)
    rules/<name>.md.tmpl   always-on rules (frontmatter: description, alwaysApply)
    partials/*.tmpl        {{define}} blocks shared by every template
  global/skills/         more skills, same namespace (optional)
  agents/<role>/skills/  more skills for a long-lived role; <role>/AGENT.md is never touched (optional)
  projects/<name>/       verbatim per-project skills/ and agents/, copied as they are (optional)
```

`trafaret` finds it, in order:

1. `--config DIR`
2. `$TRAFARET_CONFIG`
3. the nearest directory, walking up from the working directory, that contains a `manifest.yaml`

`--manifest PATH` still works: the config directory is then the directory of that file. `trafaret root` prints the
resolved config directory.

A minimal `manifest.yaml`. It installs the embedded `ship` bundle into one project and one plain skill into one home, so
it works with no templates of your own:

```yaml
engine: ">=0.3.0"              # see "Engine version gate"
vars:                          # config-wide, lowest layer
  company: Acme
  reviewers: 1
defaults:
  workspace: /work             # relative repoRoot values resolve against it
  harnesses: [claude, codex, cursor]
  vars:                        # middle layer: every project and home
    branch: main
    tracker: { kind: github }
projects:
  myproject:
    repoRoot: myproject
    bundles: [ship]            # a name from templates/bundles.yaml
    vars:                      # top layer
      lang: Go
      tracker: { repo: acme/myproject }
      gates:
        - { name: test, cmd: go test ./... }
homes:
  me:
    harness: claude
    dir: ~/.claude
    skills: [standup]
```

## How templating works

This walk-through uses [`examples/demo`](examples/demo/manifest.yaml): two projects with different vars that take the
same `ship` bundle, and two homes. The sources are in [`examples/`](examples/); what they render to is committed under
[`examples/demo/out/`](examples/demo/out/), as `out/<project-or-home>/<path as written>`.

### 1. The vars and one source

The demo manifest, in part. Vars come from three layers, and `acme-web` overrides some of them:

```yaml
vars:                       # config-wide
  company: Acme
  reviewers: 1
# …
  vars:                     # every project and home, unless overridden
    branch: main
    tracker: { kind: github }
# …
    vars:
      lang: Go
      tracker: { repo: acme/api }          # merged: kind stays github
      gates:
        - { name: test, cmd: go test ./... }
        - { name: lint, cmd: golangci-lint run }
# …
    vars:
      lang: TypeScript
      branch: develop                      # overrides defaults.vars.branch
      reviewers: 2                         # overrides vars.reviewers
      tracker: { kind: jira, host: acme.atlassian.net, key: WEB }
      gates:
        - { name: test, cmd: pnpm test }
        - { name: types, cmd: pnpm tsc --noEmit }
      e2e: pnpm playwright test
```

The source both projects render, [`examples/skills/ship/SKILL.md.tmpl`](examples/skills/ship/SKILL.md.tmpl), in part:

```
description: Use when a change in {{.Name}} is ready to land — runs the {{len .Vars.gates}} gates, writes the commit with its {{.Vars.tracker.kind}} reference, and opens the review against {{.Vars.branch}}. Applies only to the {{.Name}} repo.
…
## 1 · Gates

Every gate green before anything else:

{{range .Vars.gates}}- **{{.name}}** — `{{.cmd}}`
{{end}}
{{- if has .Vars "e2e"}}- **e2e** (slow, last) — `{{.Vars.e2e}}`
{{end}}
All of them in order, stopping at the first red: `{{.TargetRoot}}/skills/ship/scripts/gates.sh`.

{{if eq .Harness "codex" -}}
Want a second pair of eyes? Run the `gatekeeper` custom agent (`.codex/agents/gatekeeper.toml`).
{{- else -}}
Want a second pair of eyes? Dispatch the `gatekeeper` subagent — it re-runs the gates read-only.
{{- end}}
…
One commit, message `<what changed> (<ref>)`, where the ref is the issue — e.g.
`{{template "issueRef" .}}`, linked as {{template "issueURL" .}}.
…
Open the review against `{{.Vars.branch}}`; it needs {{.Vars.reviewers}} approval{{if gt .Vars.reviewers 1}}s{{end}}.
```

`{{template "issueRef" .}}` calls a define from [`examples/partials/tracker.tmpl`](examples/partials/tracker.tmpl), which
reads `.Vars.tracker`, so the skill never branches on the tracker itself.

### 2. One source, two projects

The same template rendered for `acme-api` (left, `.claude/skills/ship/SKILL.md`) and `acme-web` (right):

<table>
<tr><th align="left">acme-api</th><th align="left">acme-web</th></tr>
<tr>
<td valign="top"><pre>Acme · Go · target branch `main`.</pre></td>
<td valign="top"><pre>Acme · TypeScript · target branch `develop`.</pre></td>
</tr>
<tr>
<td valign="top"><pre>- **test** — `go test ./...`
- **lint** — `golangci-lint run`</pre></td>
<td valign="top"><pre>- **test** — `pnpm test`
- **types** — `pnpm tsc --noEmit`
- **e2e** (slow, last) — `pnpm playwright test`</pre></td>
</tr>
<tr>
<td valign="top"><pre>One commit, message `&lt;what changed&gt; (&lt;ref&gt;)`, where the ref is the issue — e.g.
`#123`, linked as https://github.com/acme/api/issues/&lt;n&gt;.</pre></td>
<td valign="top"><pre>One commit, message `&lt;what changed&gt; (&lt;ref&gt;)`, where the ref is the issue — e.g.
`WEB-123`, linked as https://acme.atlassian.net/browse/WEB-&lt;n&gt;.</pre></td>
</tr>
<tr>
<td valign="top"><pre>Open the review against `main`; it needs 1 approval.
Your open reviews: `.claude/skills/ship/scripts/my-reviews.sh`.</pre></td>
<td valign="top"><pre>Open the review against `develop`; it needs 2 approvals.
Your open reviews: `.claude/skills/ship/scripts/my-reviews.sh`.</pre></td>
</tr>
</table>

`acme-web` has an `e2e` var, so it gets a third gate; the commit reference and link come from the partial (`github` or
`jira`); the branch and the number of approvals come from the vars that `acme-web` overrides.

### 3. One source, another harness

The same skill rendered for Codex lands in `.agents/skills/ship/SKILL.md`. `{{.TargetRoot}}` is now `.agents`, and the
`{{if eq .Harness "codex"}}` branch picks the Codex wording:

```
All of them in order, stopping at the first red: `.agents/skills/ship/scripts/gates.sh`.

Want a second pair of eyes? Run the `gatekeeper` custom agent (`.codex/agents/gatekeeper.toml`).
```

The agent [`gatekeeper.md.tmpl`](examples/agents/gatekeeper.md.tmpl) is rendered once and then converted to Codex's native
TOML, `.codex/agents/gatekeeper.toml`, beside a readable `.agents/agents/gatekeeper.md` (`model: sonnet` pins a Codex
model; no `Edit` or `Write` in `tools` becomes a read-only sandbox):

```toml
name = "gatekeeper"
description = "Read-only gate runner for acme-api — runs every gate on the current commit and reports PASS, or the first failing gate with its output. Never edits code."
model = "gpt-5.6-terra"
model_reasoning_effort = "medium"
sandbox_mode = "read-only"
developer_instructions = "You are the gatekeeper for acme-api (Go). `cd /work/acme-api` first. …
```

### 4. What each feature demonstrates

| Feature | Where in the sources | Where to see it rendered |
|---|---|---|
| Var merge levels: config `vars`, `defaults.vars`, project `vars` | [`manifest.yaml`](examples/demo/manifest.yaml): the top-level `vars`, `defaults.vars`, each project's `vars` | `acme-web` overrides `branch` and `reviewers`: [`acme-api`](examples/demo/out/acme-api/.claude/skills/ship/SKILL.md) vs [`acme-web`](examples/demo/out/acme-web/.claude/skills/ship/SKILL.md) (target branch, approvals) |
| Recursive map merge | `acme-api` sets only `tracker: { repo: acme/api }` in [`manifest.yaml`](examples/demo/manifest.yaml) and keeps `kind: github` from `defaults.vars` | the GitHub link in [`acme-api`'s commit section](examples/demo/out/acme-api/.claude/skills/ship/SKILL.md) |
| Partials | [`partials/tracker.tmpl`](examples/partials/tracker.tmpl) (`issueRef`, `issueURL`), called from [`SKILL.md.tmpl`](examples/skills/ship/SKILL.md.tmpl) | `#123` and a GitHub URL in [`acme-api`](examples/demo/out/acme-api/.claude/skills/ship/SKILL.md); `WEB-123` and a Jira URL in [`acme-web`](examples/demo/out/acme-web/.claude/skills/ship/SKILL.md) |
| `has`, `range`, `if`, `len`, `gt` | [`SKILL.md.tmpl`](examples/skills/ship/SKILL.md.tmpl): `range .Vars.gates`, `has .Vars "e2e"`, `len .Vars.gates` in the description, `gt .Vars.reviewers 1` for the plural | the e2e gate and "2 approvals" only in [`acme-web`](examples/demo/out/acme-web/.claude/skills/ship/SKILL.md) |
| `.Harness` conditional | the `{{if eq .Harness "codex"}}` block in [`SKILL.md.tmpl`](examples/skills/ship/SKILL.md.tmpl) | subagent wording in [`.claude/…/SKILL.md`](examples/demo/out/acme-api/.claude/skills/ship/SKILL.md), custom-agent wording in [`.agents/…/SKILL.md`](examples/demo/out/acme-api/.agents/skills/ship/SKILL.md) |
| `.TargetRoot` | `{{.TargetRoot}}/skills/ship/scripts/…` in [`SKILL.md.tmpl`](examples/skills/ship/SKILL.md.tmpl) | `.claude/skills/…` in the [Claude copy](examples/demo/out/acme-api/.claude/skills/ship/SKILL.md), `.agents/skills/…` in the [Codex copy](examples/demo/out/acme-api/.agents/skills/ship/SKILL.md) |
| A rendered script beside a verbatim one that contains `{{ }}` | [`gates.sh.tmpl`](examples/skills/ship/scripts/gates.sh.tmpl) (rendered) and [`my-reviews.sh`](examples/skills/ship/scripts/my-reviews.sh) (no `.tmpl`, copied byte for byte) | [`gates.sh`](examples/demo/out/acme-web/.claude/skills/ship/scripts/gates.sh) with the gate commands filled in and mode `0755`; [`my-reviews.sh`](examples/demo/out/acme-api/.claude/skills/ship/scripts/my-reviews.sh) still holds its own `{{range .}}` |
| An agent converted to Codex TOML, plus a readable copy | [`agents/gatekeeper.md.tmpl`](examples/agents/gatekeeper.md.tmpl) | [`.codex/agents/gatekeeper.toml`](examples/demo/out/acme-api/.codex/agents/gatekeeper.toml), [`.agents/agents/gatekeeper.md`](examples/demo/out/acme-api/.agents/agents/gatekeeper.md), [`.claude/agents/gatekeeper.md`](examples/demo/out/acme-api/.claude/agents/gatekeeper.md) |
| A Cursor-only rule | [`rules/repo-stance.md.tmpl`](examples/rules/repo-stance.md.tmpl) | [`.cursor/rules/repo-stance.mdc`](examples/demo/out/acme-api/.cursor/rules/repo-stance.mdc); Claude and Codex skip rules |
| `sessionContext` and the managed hook | `sessionContext: ship/session-context.md` in [`bundles.yaml`](examples/bundles.yaml), and [`session-context.md.tmpl`](examples/skills/ship/session-context.md.tmpl) | [`session-context.md`](examples/demo/out/acme-api/.claude/skills/ship/session-context.md) and the [`.claude/settings.json`](examples/demo/out/acme-api/.claude/settings.json) hook that prints it |
| A plain skill copied verbatim into two homes | [`skills/standup/SKILL.md`](examples/skills/standup/SKILL.md) (no `.tmpl`) | [`me-claude`](examples/demo/out/me-claude/skills/standup/SKILL.md) and [`me-codex`](examples/demo/out/me-codex/skills/standup/SKILL.md) |
| `requires` validation | `requires: [company, lang, reviewers, branch, gates, tracker.kind]` in [`bundles.yaml`](examples/bundles.yaml) | `trafaret validate` fails with `bundle ship requires var "gates", which is not set` when a project that lists `ship` lacks one |

The committed output is tested: `TestGoldenDemo` renders `examples/demo` in memory and diffs it against `examples/demo/out`, so
the sources and what you see here cannot drift apart. After an intended change to an example or to the engine,
regenerate and review the diff like code:

```bash
go test ./... -run Golden -update
git diff examples/demo/out
```

The reference behind all of this is [`docs/template-reference.md`](docs/template-reference.md).

## Bring your own templates

The smallest useful config is a manifest and one skill template; no `bundles.yaml` is needed to install a skill by name.

```
my-config/
  manifest.yaml
  templates/
    skills/
      release-notes/
        SKILL.md.tmpl
```

`manifest.yaml`:

```yaml
engine: ">=0.3.0"
defaults:
  workspace: /work
  harnesses: [claude]
projects:
  myproject:
    repoRoot: myproject
    skills: [release-notes]
    vars:
      branch: main
```

`templates/skills/release-notes/SKILL.md.tmpl`:

```markdown
---
name: release-notes
description: Draft release notes for {{.Name}} from the commits merged since the last tag. Applies only to the {{.Name}} repo.
---

Work in `{{.RepoRoot}}`. List the commits on `{{.Vars.branch}}` since the last tag and group them by area.
```

Then:

```bash
trafaret --config my-config validate          # render everything, catch missing vars and typos
trafaret --config my-config diff myproject    # see what would be written
trafaret --config my-config install myproject # writes myproject/.claude/skills/release-notes/SKILL.md
```

To group items, add `templates/bundles.yaml` (see [Bundles](#bundles)) and list the bundle under `bundles:`.

The `templates/` directory is an **overlay** on the embedded examples: a file at the same path replaces the embedded one,
new files are added, directories merge, and `templates/bundles.yaml` replaces the embedded registry outright.
`--templates PATH` points the overlay somewhere else (it must exist); an empty directory disables the overlay.
Conventions worth following in every template: valid YAML frontmatter with a non-empty `name` and `description`, a
project-scoped description that ends with `Applies only to the {{.Name}} repo.`, and `{{.TargetRoot}}` for any in-repo path
to a skill or agent file. The full contract is in [`docs/template-reference.md`](docs/template-reference.md).

## Manifest

```yaml
engine: ">=0.3.0"
vars: {}                    # config-wide, lowest layer
defaults:
  workspace: /work          # a relative repoRoot resolves against it
  harnesses: [claude, codex, cursor]
  vars: {}                  # middle layer
projects:
  <name>:
    repoRoot: path
    harnesses: [..]         # default: defaults.harnesses
    skills: [..]
    agents: [..]
    rules: [..]
    bundles: [..]
    disabled: [..]          # bundles declared but switched off (pruned)
    vars: {}                # top layer
homes:
  <name>:
    harness: claude|codex|cursor
    dir: ~/.claude          # ~ is expanded; must be absolute after that
    skills: [..]            # omitted = every global skill
    skip: [..]
    agents: [..]
    rules: [..]
    bundles: [..]
    vars: {}
```

| Field | Meaning |
|---|---|
| `engine` | optional trafaret version constraint, see [Engine version gate](#engine-version-gate) |
| `vars`, `defaults.vars`, entry `vars` | the three layers of values templates read as `.Vars` |
| `defaults.workspace` | absolute path that a relative `repoRoot` resolves against |
| `defaults.harnesses`, project `harnesses` | which harnesses a project is rendered for: `claude`, `codex`, `cursor` |
| project `skills` / `agents` / `rules` | names from the source namespace |
| `bundles`, `disabled` | bundles to install; `disabled` switches one off without forgetting it (the next full install prunes its artifacts) |
| home `harness`, `dir` | the harness and its user directory; one home belongs to one harness |
| home `skills` | omitted: every **global** skill (`global/skills` and `agents/*/skills`); a list: exactly those (plus bundles); `[]`: none |
| home `skip` | names (skills, agents, rules) this home does not take |

A project or home installs the union of its own lists and the contents of its active bundles.

**Vars merge.** A project or home sees `vars` < `defaults.vars` < its own `vars`. Maps merge recursively key by key;
scalars and lists replace; a `null` value deletes the key, so `e2e: null` removes an inherited `e2e`. The result is a
deep copy per entry, so nothing leaks between projects.

**Removed in 0.3.** The typed project fields `tickets`, `run`, `gates`, `docs`, `stack`, `tools`, `vcs`, `e2e`;
`targets` (now `harnesses`); and a home's `skillsDir` (now `harness` + `dir`). They are rejected with a migration hint,
not ignored. To move a v0.2 config see [`docs/spec.md`](docs/spec.md#migration-of-a-v02-config) and
[`docs/onboarding.md`](docs/onboarding.md).

## Harnesses and where files land

One source, placed by a small adapter table per harness and per scope:

| item | claude | codex | cursor |
|---|---|---|---|
| project skill | `.claude/skills/<n>/` | `.agents/skills/<n>/` | shares `.claude/skills` |
| project agent | `.claude/agents/<n>.md` | `.codex/agents/<n>.toml` + readable `.agents/agents/<n>.md` | shares `.claude/agents` |
| project rule | skipped | skipped | `.cursor/rules/<n>.mdc` |
| home skill | `<dir>/skills/<n>/` | `<dir>/skills/<n>/` | `<dir>/skills/<n>/` |
| home agent | `<dir>/agents/<n>.md` | `<dir>/agents/<n>.toml` | `<dir>/agents/<n>.md` |
| home rule | skipped | skipped | `<dir>/rules/<n>.mdc` |
| session hook | `.claude/settings.json` | none | via `.claude/settings.json` |

Cursor reads `.claude/skills` and `.claude/agents` natively, so the shared tree is rendered once and Cursor contributes
only its rules; never create `.cursor/skills` or `.cursor/agents`, which would load every skill twice. Templates see
which copy they are: `{{.Harness}}` and `{{.TargetRoot}}` (`.claude`, `.agents`, `.cursor`, or the absolute home dir).
Adding a harness is adding one adapter value in `internal/engine/adapters.go`; the core does not change.

## Write, track, prune

- **Idempotent writes.** Each file is `install`, `update`, `skip` or `remove`; `--force` rewrites anyway. A rendered file
  that starts with `#!` is written executable (`0755`).
- **Tracked.** A project records what it wrote in `.claude/ao-manifest.json`. A home records per directory it writes
  into (`<dir>/skills/`, `<dir>/agents/`, `<dir>/rules/`) in a `.ao-manifest.json`. Only tracked paths are ever pruned,
  never a directory scan, so the harness's own files are never touched. The `ao-` names are historical; renaming them
  would orphan existing installs.
- **Pruned.** A full install deletes what the current render no longer produces, which is how a bundle moved to
  `disabled:` leaves the repo. `--what` and `--target` partial installs never prune other bundles' or other harnesses'
  files.
- **Linted.** The frontmatter of every emitted `.md` and `.mdc` must parse as YAML; `SKILL.md` needs a non-empty `name`
  and `description`; an unquoted value may not contain ` #` (YAML drops the rest of the line there).

**Edit the template or the manifest, then reinstall.** The rendered `.claude/`, `.agents/`, `.codex/` and `.cursor/` trees
and the home directories' managed files are build artifacts; never hand-edit them.

## Bundles

A **bundle** is a named set of skills, agents and rules. A project or home lists the bundles it wants under `bundles:`. The
registry is data, not code: `templates/bundles.yaml`, a map from bundle name to its entry.

```yaml
ship:
  skills: [ship]
  agents: [gatekeeper]
  rules: [repo-stance]
  requires: [company, lang, reviewers, branch, gates, tracker.kind]
  sessionContext: ship/session-context.md
```

| Field | Meaning |
|---|---|
| `skills` | names from the skill namespace: `templates/skills`, `global/skills`, `agents/*/skills` and the embedded examples |
| `agents` | `templates/agents/<name>.md[.tmpl]` |
| `rules` | `templates/rules/<name>.md[.tmpl]`; rendered to `.cursor/rules/<name>.mdc` for Cursor, skipped for Claude and Codex |
| `requires` | var paths (dotted, e.g. `tracker.kind`) the bundle's templates read. `trafaret validate` checks that each exists in the merged vars of every project and home that lists the bundle, disabled ones included |
| `sessionContext` | a file under `.claude/skills/` printed at every session start by a `SessionStart` hook that trafaret manages in the project's `.claude/settings.json`. At most one active bundle per project may set it; trafaret owns exactly one hook entry, merged additively and removed when the bundle goes away |

All fields are optional. An unknown field is an error (`bundles.yaml` is strict), and an unknown bundle name in a
project or home is an error.

**Shadowing.** A config's `templates/bundles.yaml` *replaces* the embedded registry; it is not merged. The bundles that
exist for a config are exactly the ones its own `bundles.yaml` lists. A config with none gets the embedded one.

### Shipped example

| Bundle | Installs |
|---|---|
| `ship` | skill `ship` (gates, commit reference, review; with `scripts/gates.sh` and `scripts/my-reviews.sh`), agent `gatekeeper`, Cursor rule `repo-stance`; `requires: [company, lang, reviewers, branch, gates, tracker.kind]`; `sessionContext` set |

Its sources are in [`examples/`](examples/), and a plain skill, `standup`, is there too for homes. Copy what you need into
your config's `templates/` and write your own.

## Usage

```bash
trafaret list                          # projects (harnesses, bundles, skills) and homes (harness, dir, skills)
trafaret validate                      # check the manifest and render every declared item (catches missing vars, typos)
trafaret diff myproject                # dry-run: per-file install/update/skip/remove
trafaret install myproject             # render + write into myproject/.claude, .agents, .codex, .cursor
trafaret install myproject --target claude        # Claude only: overrides the project's harnesses for this run
trafaret install --all                 # every home and every project
trafaret install myproject --what ship            # only the named bundle(s), a subset of the project's; no pruning
trafaret install myproject --force --dry-run --json
trafaret install me                    # render + write into one home's harness dir (see homes: in the manifest)
trafaret --config ~/my-config diff myproject      # global flags may come before the command
trafaret root                          # the resolved config directory
trafaret serve                         # the local operator manual (see below)
trafaret skill                         # the agent-facing reference for this CLI
trafaret version
```

`--target claude|cursor|codex|both|all` overrides a project's harnesses and selects only the homes of that harness.
`--verbose` also lists skipped files; `--json` prints machine-readable output.

## Engine version gate

A manifest can pin the trafaret it was written for:

```yaml
engine: ">=0.3.0"      # or an exact "0.3.1"; comma-separate for ranges: ">=0.3.0, <1.0.0"
```

If the binary's version does not satisfy it, every command that reads the manifest exits with a clear error. The version
is stamped at build time (`make build` uses `git describe`); an unstamped `dev` build always passes. An absent field
means no check; a constraint trafaret cannot parse is an error. A v0.3 manifest should pin `>=0.3.0`.

## Operator manual (local site)

```bash
make site               # http://127.0.0.1:4477 — opens in the browser
trafaret serve --addr 127.0.0.1:4477
```

A multi-page app (overview, live state, catalog, guardrails, how-to, where-to-edit) in two languages: `/en/<page>` and
`/ru/<page>`. It renders every page from the config directory on **every refresh**, shows the live per-project and per-home sync
status (the same comparison as `trafaret diff`), the source catalog and bundle table, and the how-to tables. Localhost
only; mermaid is vendored (`internal/site/assets/`), so it works offline.

## Layout of this repository

| Path | What |
|---|---|
| `main.go` | calls `cmd.Execute()` |
| `cmd/` | the cobra commands, one file per command (`install`, `list`, `validate`, `serve`, …); `skill.md` is the embedded agent reference printed by `trafaret skill`; `help.go` the usage screen; `version.go` the build-stamped `version` |
| `internal/engine/` | the core: manifest and vars merge, the bundle registry, the source namespace, render, the harness adapters (`adapters.go`), install, prune, the managed session hook, config-directory resolution, the templates overlay, the `engine:` version gate |
| `internal/site/` | the live operator site (`trafaret serve`): data collection, page skeleton, EN/RU strings, vendored mermaid in `assets/` |
| `examples/` | the embedded example set (`go:embed`): `bundles.yaml`, `skills/`, `agents/`, `rules/`, `partials/`; and `examples/demo`, the runnable demo config with its committed output in `out/` (not embedded) |
| `docs/spec.md` | design and rationale, the 0.2 to 0.3 migration |
| `docs/template-reference.md` | the template contract: data, funcs, vars merge, adapters, lint |
| `docs/onboarding.md` | how to onboard a project and a home |

See [CONTRIBUTING.md](CONTRIBUTING.md) to work on it and [SECURITY.md](SECURITY.md) to report a vulnerability.

## License

MIT — see [LICENSE](LICENSE).
