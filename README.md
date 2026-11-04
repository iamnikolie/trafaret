# trafaret

Render a project-parameterized library of agent skills, subagents and rules into your repositories'
native **Claude Code**, **Cursor** and **Codex** locations.

trafaret is a generic engine. You write the templates once (Go `text/template`, with tokens such as the
project name, branch, gate commands, ticket board, docs layout) and group them into **bundles**; each
project's token values live in a `manifest.yaml`. `trafaret install <project>` renders the bundles the
project asked for into that repo. Writes are sha256-idempotent: unchanged files are skipped.

The binary embeds only a few **examples** — they show the template format and every token family.
Your real templates and your own bundle list live in your **config directory**, in its `templates/`
overlay (see [Bring your own templates](#bring-your-own-templates)).

## Install

```bash
go install github.com/iamnikolie/trafaret@latest
```

or from a checkout:

```bash
make install            # builds ./trafaret and symlinks ~/.local/bin/trafaret -> ./trafaret
```

The example templates are embedded in the binary — there is nothing else to install and it runs from
any directory.

## The config directory

Everything that is *yours* lives in one directory, the **config directory** — the directory that holds
`manifest.yaml`:

```
my-config/
  manifest.yaml        projects, homes and their token values (required)
  templates/           your templates and bundles; overlay on the embedded examples (optional)
    bundles.yaml         the bundle registry (shadows the embedded one)
    skills/<name>/       SKILL.md.tmpl + any other files of a skill
    agents/<name>.md.tmpl
    cursor/rules/<name>.mdc.tmpl
  global/skills/       skills copied verbatim into every home (optional)
  agents/<role>/       role agents: AGENT.md boot doc + skills/ copied into the homes (optional)
  projects/<name>/     per-project verbatim skills/agents, copied as-is (optional)
```

`trafaret` finds it, in order:

1. `--config DIR`
2. `$TRAFARET_CONFIG`
3. the nearest directory, walking up from the working directory, that contains a `manifest.yaml`

`--manifest PATH` still works: the config directory is then the directory of that file.
`trafaret root` prints the resolved config directory.

A minimal `manifest.yaml` (it installs the embedded `example` bundle, so it works with no templates of
your own):

```yaml
engine: ">=0.2.0"            # optional, see "Engine version gate"
defaults:
  workspace: /work           # relative repoRoot values resolve against it
  targets: [claude, cursor, codex]
projects:
  myproject:
    repoRoot: myproject
    bundles: [example]       # a name from templates/bundles.yaml
    tickets:
      cli: fibery
      config: acme
      space: "Acme development"
      host: acme.example.com
      boards: { tasks: Tasks, bugs: Bugs }
      states: { backlog: Backlog, todo: "To do", inProgress: "In progress", inReview: "In review", done: "To test" }
      priority: [Critical, High, Medium, Low]
      refScheme: { task: "Task-N", bug: "Bug-N" }
      dependsOn: "Depends on: <ref>"
```

See [`docs/onboarding.md`](docs/onboarding.md) for the full block and
[`docs/token-reference.md`](docs/token-reference.md) for every token.

## Bring your own templates

The smallest useful config: a manifest, a bundle registry and one skill template.

```
my-config/
  manifest.yaml
  templates/
    bundles.yaml
    skills/
      release-notes/
        SKILL.md.tmpl
```

`manifest.yaml`:

```yaml
defaults:
  workspace: /work
  targets: [claude]
projects:
  myproject:
    repoRoot: myproject
    bundles: [release]
    vcs:
      defaultBranch: main
```

`templates/bundles.yaml` — defines exactly the bundles this config has:

```yaml
release:
  skills: [release-notes]
```

`templates/skills/release-notes/SKILL.md.tmpl`:

```markdown
---
name: release-notes
description: Draft release notes for {{.Name}} from the commits merged since the last tag. Applies only to the {{.Name}} repo at {{.RepoRoot}}.
---

Project: {{.Name}} — stop if the working directory is not {{.RepoRoot}}.

List the commits on `{{.VCS.DefaultBranch}}` since the last tag and group them by area.
```

Then:

```bash
trafaret --config my-config validate          # render everything, catch token typos
trafaret --config my-config diff myproject    # see what would be written
trafaret --config my-config install myproject # writes myproject/.claude/skills/release-notes/SKILL.md
```

Conventions worth following in every template (the engine lints the first, the second keeps
multi-project workspaces unambiguous): valid YAML frontmatter with a non-empty `name` and `description`,
and a description that ends with `Applies only to the {{.Name}} repo at {{.RepoRoot}}.` Use
`{{.TargetRoot}}` for any in-repo path to a skill or agent file.

## Usage

```bash
trafaret list                          # projects and homes in the manifest + their targets/bundles
trafaret validate                      # check the manifest and render every template (catches token typos)
trafaret diff myproject                # dry-run: per-file install/update/skip/remove
trafaret install myproject             # render + write into myproject/.claude, .agents, .codex, .cursor
trafaret install myproject --target claude        # Claude only
trafaret install --all                 # every project and every home
trafaret install myproject --what example         # only the named bundle(s), a subset of the project's
trafaret install myproject --force --dry-run --json
trafaret install myhome                # copy global/skills + agents/*/skills into one home's skills dir
trafaret --config ~/my-config diff myproject      # global flags may come before the command
trafaret root                          # the resolved config directory
trafaret serve                         # the local operator manual (see below)
trafaret version
```

## Model

```
embedded examples ───┐   (Go text/template, tokenized)
<config>/templates ──┤   your templates + bundles.yaml: same path wins, new files are added
<config>/projects/<name>/{skills,agents}/ ── per-project verbatim files
<config>/manifest.yaml ── per-project token values
     │  trafaret install <project>
     ▼
<repoRoot>/.claude/{agents,skills}/   ── GENERATED artifacts    ← never hand-edit
<repoRoot>/.cursor/rules/                 (Cursor reads .claude/skills + .claude/agents via compat)
<repoRoot>/.agents/{skills,agents}/   ── Codex skills + readable agent contracts
<repoRoot>/.codex/agents/*.toml       ── native Codex custom agents

<config>/global/skills + <config>/agents/<role>/skills ── copied verbatim into each home's skills dir
```

Templates render **once per target**: `{{.TargetRoot}}` resolves to `.claude` / `.agents` so in-repo
references point at the tree the file actually lands in. Rendered frontmatter is YAML-linted at render
time (parse + non-empty name/description + the unquoted-` #` truncation trap).

**Edit the template or the manifest, then reinstall.** The installed `.claude/`, `.cursor/`, `.agents/`
and `.codex/` trees are build artifacts.

## Templates overlay

The binary carries a small example set. Your config's `templates/` directory is layered on top of it:

```
<config>/templates/skills/release-notes/SKILL.md.tmpl     a new skill
<config>/templates/skills/release-notes/checklist.md.tmpl  another file next to it (every file directly in
                                                           a skill's directory is rendered with the
                                                           project's tokens)
<config>/templates/skills/example-tickets/SKILL.md.tmpl   replaces the embedded example at that path
```

Directories are merged and a file in the overlay wins over the embedded file at the same path.
`--templates PATH` points the overlay somewhere else (it must exist); an empty directory disables the
overlay. `trafaret serve` and `trafaret validate` see the overlay too.

## Engine version gate

A manifest can pin the trafaret it was written for:

```yaml
engine: ">=0.2.0"      # or an exact "0.2.1"; comma-separate for ranges: ">=0.2.0, <1.0.0"
```

If the binary's version does not satisfy it, every command that reads the manifest exits with a clear
error. The version is stamped at build time (`make build` uses `git describe`); an unstamped `dev` build
always passes. An absent field means no check; a constraint trafaret cannot parse is an error.

## Operator manual (local site)

```bash
make site               # http://127.0.0.1:4477 — opens in the browser
trafaret serve --addr 127.0.0.1:4477
```

A multi-page app (overview, live state, catalog, guardrails, how-to, where-to-edit) in two languages —
`/en/<page>` and `/ru/<page>`. It renders every page from the config directory on **every refresh**,
shows the live per-project sync status (the same comparison as `trafaret diff`), the template catalog
and bundle table, and the how-to tables. Localhost only; mermaid is vendored (`docs/site-assets/`), so
it works offline.

## Bundles

A **bundle** is a named set of templates. A project lists the bundles it wants (`bundles:` in the
manifest); `disabled:` switches one off without forgetting it (the next full install prunes its
artifacts). The registry is data, not code: `templates/bundles.yaml`, a map from bundle name to the
templates it installs.

```yaml
release:
  skills: [release-notes, changelog]
  agents: [release-checker]
  cursorRules: [release-stance]
  requires: [tickets]
  sessionContext: release-notes/session-context.md
```

| Field | Meaning |
|---|---|
| `skills` | directories under `templates/skills/`. Every file directly in the directory is rendered with the project's tokens; the `.tmpl` suffix is dropped. Installed to `.claude/skills/<name>/` (Claude, Cursor) and `.agents/skills/<name>/` (Codex) |
| `agents` | `templates/agents/<name>.md.tmpl`. Rendered to `.claude/agents/<name>.md` (Claude, Cursor); Codex gets `.agents/agents/<name>.md` plus a native `.codex/agents/<name>.toml` |
| `cursorRules` | `templates/cursor/rules/<name>.mdc.tmpl`, rendered to `.cursor/rules/<name>.mdc` |
| `requires` | which manifest blocks the bundle's templates read, so `validate` can check them: `tickets` (the ticket protocol and the `run:` block) and/or `db` (`tools.db`). Omit for a bundle that reads only the bare project fields |
| `sessionContext` | a file under `.claude/skills/` (e.g. `release-notes/session-context.md`). A SessionStart hook that trafaret manages in the project's `.claude/settings.json` prints it at every session start. At most one active bundle per project may set it; trafaret owns exactly one hook entry, merged additively and removed when the bundle goes away |

All fields are optional. An unknown bundle name in a project, or an unknown `requires` entry, is an
error.

**Shadowing.** A config's `templates/bundles.yaml` *replaces* the embedded registry — it is not merged
with it. The bundles that exist for a config are exactly the ones its own `bundles.yaml` lists. A config
with no `bundles.yaml` of its own gets the embedded examples.

### Shipped examples

| Bundle | Installs |
|---|---|
| `example` | skill `example-tickets` (ticket, run-profile, gate, docs and memory tokens), agent `example-reviewer`, Cursor rule `example-rule`; `requires: [tickets]`; `sessionContext` set |
| `example-db` | skill `example-db` (read-only database access through the CLI named in `tools.db.cli`); `requires: [db]`, so it needs no ticket protocol |

They exist to show the format. Copy what you need into your config's `templates/` and write your own.

## Layout of this repository

| Path | What |
|---|---|
| `main.go` | the CLI: load manifest → render per target → lint → write (sha256-idempotent); the bundle registry loader |
| `config.go` | config-directory resolution, the embedded-template overlay, the engine version gate |
| `site.go`, `site_page.go`, `site_strings.go` | the live operator site (`trafaret serve`) |
| `templates/bundles.yaml` | the embedded bundle registry (examples) |
| `templates/skills/`, `templates/agents/`, `templates/cursor/rules/` | the embedded example templates (embedded with `go:embed`) |
| `docs/token-reference.md` | the template contract — every `{{.Token}}` |
| `docs/onboarding.md` | how to onboard a project |
| `docs/spec.md` | design and rationale |

## License

MIT — see [LICENSE](LICENSE).
