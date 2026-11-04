# trafaret — render one agent library into many repos (agent reference)

`trafaret` renders a shared library of agent skills, subagents and rules —
Go templates plus per-project token values — into each repository's native
Claude Code, Cursor and Codex locations. Writes are sha256-idempotent: an
unchanged file is not touched. What it writes is a build artifact; the source
is the template or the manifest, never the rendered tree.

## Shape of an invocation

```
trafaret [--config DIR] <command> [project|home] [flags]
```

Flags work before or after the command: `trafaret --config . validate` and
`trafaret validate --config .` are the same call.

The **config directory** holds everything that is the user's: `manifest.yaml`
plus optional `templates/`, `global/`, `agents/`, `projects/`. It is found, in
order, by `--manifest PATH` (its directory), `--config DIR`, `$TRAFARET_CONFIG`,
then the nearest parent of the working directory that has a `manifest.yaml`.
`trafaret root` prints the one it resolved. `version`, `skill` and `help` need
no config.

## Commands

| Command | Purpose |
|---|---|
| `list [--json]` | projects and homes in the manifest, with targets and bundles |
| `validate` | check the manifest and render every template; exit 1 when anything is wrong |
| `diff <project\|home>` | dry-run: per-file `install` / `update` / `skip` / `remove` |
| `install <project\|home>` | render and write; prunes artifacts of bundles moved to `disabled:` |
| `install --all` | every project and every home in the manifest |
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
| `--target claude\|cursor\|codex\|both\|all` | override the manifest's targets for this run |
| `--what a,b` | install only these bundles (a subset of the project's); skips pruning |
| `--force` | rewrite even sha256-equal files |
| `--dry-run` | render and report, write nothing |
| `--verbose`, `-v` | also list `skip` lines |
| `--json` | machine-readable output (list, install, diff) |
| `--addr host:port` | serve: listen address; a bare port binds localhost |
| `--version`, `-V` | print the version, same as the `version` command |

## What gets written

Per project, from the bundles it lists in `bundles:` minus `disabled:`:

| Target | Lands in |
|---|---|
| `claude`, `cursor` | `<repoRoot>/.claude/skills/<name>/…`, `.claude/agents/<name>.md` (Cursor reads this tree natively), `.cursor/rules/<name>.mdc` for Cursor rules |
| `codex` | `.agents/skills/<name>/…`, `.agents/agents/<name>.md`, native `.codex/agents/<name>.toml` |

A home (`homes:` in the manifest) is a per-user harness directory such as
`~/.claude`. It receives `global/skills/*` and `agents/<role>/skills/*` from the
config directory, verbatim, with no token pass. Skill names must be unique
across all of those sources.

What trafaret wrote is recorded in `.claude/ao-manifest.json` in the repo (and
`.ao-manifest.json` in a home's skills dir). Only recorded paths are ever
pruned, so files it did not write are never touched.

## The overlay rule

The binary embeds a small set of **example** templates. The config's
`templates/` directory is layered on top:

- a file at the same path replaces the embedded one;
- new files are added;
- directories are merged;
- `templates/bundles.yaml` **replaces** the embedded registry — it is not
  merged. A config with its own `bundles.yaml` has exactly the bundles it lists.

```
<config>/templates/bundles.yaml
<config>/templates/skills/<name>/SKILL.md.tmpl      (+ any other file in that dir, all rendered)
<config>/templates/agents/<name>.md.tmpl
<config>/templates/cursor/rules/<name>.mdc.tmpl
```

## bundles.yaml

A bundle is a named set of templates; a project opts in by name.

```yaml
release:
  skills: [release-notes, changelog]     # dirs under templates/skills/
  agents: [release-checker]              # templates/agents/<name>.md.tmpl
  cursorRules: [release-stance]          # templates/cursor/rules/<name>.mdc.tmpl
  requires: [tickets]                    # manifest blocks the templates read: tickets, db
  sessionContext: release-notes/session-context.md
```

All fields are optional. `requires` lets `validate` check the matching manifest
blocks (`tickets`: the ticket protocol and `run:`; `db`: `tools.db`).
`sessionContext` is a file under `.claude/skills/` that a SessionStart hook in
the project's `.claude/settings.json` prints at every session start; trafaret
owns exactly one hook entry, merged additively, and at most one active bundle
per project may set it.

## manifest.yaml

```yaml
engine: ">=0.2.0"              # optional: refuse older binaries
defaults:
  workspace: /work             # relative repoRoot values resolve against it
  targets: [claude, cursor, codex]
projects:
  myproject:
    repoRoot: myproject
    bundles: [example]
    disabled: []               # declared but switched off; the next install prunes them
    vcs: { defaultBranch: main }
homes:
  claude:
    skillsDir: ~/.claude/skills
    skip: [codex-only-skill]
```

Project fields are the template tokens: `{{.Name}}`, `{{.RepoRoot}}`,
`{{.VCS.DefaultBranch}}`, `{{.Tickets.Space}}`, `{{.Gates.Packages}}`, and so
on. `{{.TargetRoot}}` renders as `.claude` or `.agents` for the tree the file
lands in; use it for any in-repo path to a skill or agent file. An unknown token
is a render error, not an empty string.

## Rendering rules

- Frontmatter of every rendered `.md` / `.mdc` is linted: valid YAML, a
  non-empty `name` and `description` in `SKILL.md`, and no unquoted value
  containing ` #` (YAML silently drops the rest of the line).
- Skill descriptions should end with `Applies only to the <name> repo at
  <path>.` and the body should open with a `Project:` guard line: same-named
  skills load side by side in multi-root workspaces.
- Rendering is byte-stable. After any template or manifest edit run `diff` and
  read the `update` lines before installing.

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
trafaret install myproject --what release

# feed a script
trafaret list --json
trafaret install --all --dry-run --json

# sync a home's skills directory
trafaret install claude
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
| `install: name a project or home, or pass --all` | nothing selected |
| `no such project or home: <name>` | typo, or the name is not in the manifest |
| `invalid --target (claude\|cursor\|codex\|both\|all): <x>` | unknown target |
| `bundle "<x>" is not in templates/bundles.yaml` | a config's own registry shadows the embedded one; add the bundle there |
| `render error: … can't evaluate field X …` | a template reads a token that does not exist (a typo, or a field the project struct lacks) |
| `frontmatter … contains ' #' in an unquoted value` | quote the value or drop the `#` |
| `N problem(s)` after `validate` output | each line above it names a project and the problem |
