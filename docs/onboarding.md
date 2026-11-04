# Onboarding a project or a home

How to bring a repository (a **project**) or a user-level harness directory (a **home**) under trafaret v0.3. The
engine knows harness layouts and nothing else, so there is no per-project checklist of tickets, gates or docs: you
decide what your templates read, write that as `vars`, and pick the bundles that use it.

Coming from a 0.2 config? Go to [Migrating from a v0.2 config](#migrating-from-a-v02-config) first.

If you have no templates of your own yet, the embedded `ship` bundle and the demo config
([`examples/demo`](../examples/demo/manifest.yaml)) are a complete working example, and every step below is
illustrated with them. Try the demo read-only:

```bash
trafaret --config examples/demo validate
trafaret --config examples/demo install acme-api --dry-run
```

The demo's homes are scratch directories under `/tmp/trafaret-demo/` and its projects resolve under `/work`, so it never
touches a real harness; `validate`, `list`, `diff` and `install --dry-run` show everything it would write.

## Onboard a project

### 1. Pick the harnesses

`harnesses` says which tools the repo is rendered for. It defaults to `defaults.harnesses`.

| Harness | The project gets |
|---|---|
| `claude` | `.claude/skills/`, `.claude/agents/`, and the session hook in `.claude/settings.json` |
| `codex` | `.agents/skills/`, `.codex/agents/*.toml`, and readable agent copies in `.agents/agents/` |
| `cursor` | `.cursor/rules/*.mdc`; Cursor reads `.claude/skills` and `.claude/agents` natively, so it adds no skill or agent copy |

Listing `claude` and `cursor` together costs nothing: the shared `.claude` tree is written once, and `cursor` adds the
rules. See [`template-reference.md`](template-reference.md#where-each-item-lands) for the full table.

### 2. Write the vars

List what the templates read, then set each value at the lowest level that is true for everyone who shares it. The
demo's `ship` skill reads these (find them in any template with `.Vars.` in the source):

| Var | Read by | Set in the demo at |
|---|---|---|
| `company`, `reviewers` | `ship` | the manifest's top-level `vars` (config-wide); `acme-web` overrides `reviewers` |
| `branch` | `ship`, its session context, the `repo-stance` rule | `defaults.vars` (every project); `acme-web` overrides it |
| `tracker.kind` | `ship`, the `issueRef` and `issueURL` partials | `defaults.vars` (`github`); `acme-web` sets `jira` |
| `tracker.repo`, or `tracker.host` and `tracker.key` | the two partials | each project's own `tracker`, merged with the default `kind` |
| `lang`, `gates` | `ship`, `gates.sh`, `gatekeeper` | each project's own `vars` |
| `e2e` | `ship` and `gates.sh`, only when present | `acme-web` only; `acme-api` simply omits it |

That is this project block (from [`examples/demo/manifest.yaml`](../examples/demo/manifest.yaml)):

```yaml
  acme-web:
    repoRoot: acme-web
    bundles: [ship]
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

The three layers merge as maps (a project that sets only `tracker: { repo: acme/api }` keeps `kind` from the
defaults), lists and scalars replace, and `null` deletes an inherited key (`e2e: null`). The rules, with a worked
example, are in [`template-reference.md`](template-reference.md#vars-and-the-merge). Keep values out of the templates;
anything that differs between projects, or that you would not want to publish, is a var.

### 3. Choose the bundles

A bundle is a named set of skills, agents and rules, defined in `templates/bundles.yaml`. List the ones the project
takes under `bundles:`, or name items directly under `skills:`, `agents:` and `rules:` (the project installs the union).
Start small: one bundle, then add. `disabled: [name]` keeps a bundle declared but switched off, and the next full
install prunes what it wrote.

### 4. Declare `requires`

A bundle's `requires` lists the var paths its templates read unconditionally:

```yaml
ship:
  skills: [ship]
  agents: [gatekeeper]
  rules: [repo-stance]
  requires: [branch, gates, tracker.kind]
  sessionContext: ship/session-context.md
```

`trafaret validate` checks every project and home that lists the bundle (disabled ones too) and reports the missing
path: `bundle ship requires var "gates", which is not set (vars: / defaults.vars / the entry's own vars:)`. Vars a
template reads only behind `has` (the demo's `e2e`) do not belong in `requires`. The check is existence only.
`validate` also renders every declared item, so a var a template reads that `requires` forgot still fails there, with
the file and line.

### 5. Validate, diff, install

```bash
trafaret validate                # render every item of every project and home; catches missing vars and typos
trafaret diff acme-web           # per-file install / update / skip / remove, writes nothing
trafaret install acme-web        # render and write
```

Then open one rendered file (for the demo, `acme-web/.claude/skills/ship/SKILL.md`) and confirm the values are the
ones you meant. Never edit rendered files: they are build artifacts and the next install overwrites them. The
committed demo output under [`examples/demo/out/`](../examples/demo/out/) is what a correct result looks like.

### 6. Session context (optional)

A bundle with `sessionContext` ships a short file that a `SessionStart` hook prints at every session start, so the
agent begins each session knowing how work lands in the repo. The demo's
[`session-context.md.tmpl`](../examples/skills/ship/session-context.md.tmpl) renders to a three-line
`<ship>` block. trafaret owns exactly one hook entry in `.claude/settings.json`, merges it additively and removes it
when the bundle is disabled. Only one active bundle per project may set it. Nothing else in `settings.json` is
touched.

### What to track in git

**Nothing trafaret renders.** Your config is the source of truth and `trafaret install` is a precondition for working
in a project, so a committed copy of the output is only ever a second, lagging one: one template change would land as
the same diff in ten repositories and wait to be committed ten times. Ignore the rendered trees:

```gitignore
.claude/agents/*
.claude/skills/*
!.claude/skills/README.md
.claude/ao-manifest.json
.cursor/rules/<rendered-rule>.mdc     # one line per rule your bundles render
.agents/
.codex/
```

What stays tracked is what a human wrote: `.claude/settings.json` (hooks, permissions; trafaret only adds and removes
its own single `SessionStart` entry), any catalog `README.md`, and Cursor rules trafaret does not render. Note the
shape of the first two lines: `dir/*` rather than `dir/`, so a hand-authored file inside a rendered directory can
still be re-included. Git cannot re-include anything under a directory excluded outright.

`.claude/ao-manifest.json` is trafaret's own ledger of what it wrote, read to delete artifacts a later render dropped.
It is build state, not a declaration of what the repo contains, so it is ignored with the rest.

This assumes every environment that uses the rendered files has `trafaret` and your config directory. If one ever has
to work from a bare clone (a cloud session, someone else's machine), that environment needs a bootstrap step, or the
tracking rule flips back for `.claude/`.

If a project does track rendered artifacts and uses Cursor, whitelist only its rules directory and never create
`.cursor/skills` or `.cursor/agents`: Cursor does not dedupe same-named skills, so a second copy loads everything twice.

```gitignore
.cursor/*
!.cursor/rules/
```

## Onboard a home

A home is a user-level harness directory: `~/.claude`, `~/.codex` or `~/.cursor`, or any other directory a harness
reads. It goes through the same pipeline as a project; only the layout differs. From the demo:

```yaml
homes:
  me-claude:
    harness: claude
    dir: ~/.claude
    skills: [standup]
  me-codex:
    harness: codex
    dir: ~/.codex
    skills: [standup]
```

- `harness` is one of `claude`, `codex`, `cursor`; `dir` is the harness's user directory (`~` is expanded and the result
  must be absolute). A home belongs to exactly one harness, so one machine with three tools has three homes.
- What it receives, per harness: skills in `<dir>/skills/<name>/`, agents in `<dir>/agents/<name>.md` (`.toml` for Codex),
  and, for Cursor only, rules in `<dir>/rules/<name>.mdc`. There is no hook in a home.
- **`skills:` omitted** installs every global skill: everything under `<config>/global/skills` and
  `<config>/agents/*/skills`. A skill from `templates/skills` or the embedded examples reaches a home only when named in
  `skills:` or through a bundle. `skills: []` installs none. The demo's `standup` is a plain skill copied verbatim into
  both homes ([claude](../examples/demo/out/me-claude/skills/standup/SKILL.md),
  [codex](../examples/demo/out/me-codex/skills/standup/SKILL.md)).
- `skip: [name]` drops names a home should not take (a skill that only works in one harness is skipped by the other
  homes). `agents`, `rules` and `bundles` work as in a project; `vars` are merged the same three-layer way, so a home
  template can use `.Vars`, and its `.TargetRoot` is the absolute home directory.
- Name a home differently from every project: `trafaret install <name>` takes either, and a collision is reported by
  `validate`.
- trafaret tracks what it wrote per directory (`<dir>/skills/.ao-manifest.json`, and likewise `agents/` and `rules/`)
  and prunes only those paths: the harness's own files in `<dir>` are never touched.
- `--target claude` (or `codex`, `cursor`) installs only the homes of that harness, and for projects overrides the
  harnesses for the run.

Run `trafaret diff <home>` before the first install, read the list, and only then `trafaret install <home>`.

## Migrating from a v0.2 config

The typed project fields (`tickets`, `run`, `gates`, `docs`, `stack`, `tools`, `vcs`, `e2e`) and `targets` and
`skillsDir` are gone, and the manifest is rejected until they are moved. The steps are in
[`spec.md`, Migration of a v0.2 config](spec.md#migration-of-a-v02-config): move the blocks under `vars:`, rewrite
`.Tickets.Space` to `.Vars.tickets.space`, move computed values into partials, rename `targets` to `harnesses`, turn
`skillsDir` into `harness` and `dir`, rename `cursorRules` to `rules`, and set `engine: ">=0.3.0"`.

Output paths and track files are unchanged, so the check is mechanical: run `trafaret diff --all` and expect every
unchanged file to be `skip`. Read any `update` or `remove` line before installing.
