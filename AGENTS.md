# trafaret — agent notes

`trafaret` is a Go CLI that renders a library of agent skills, subagents and rules into Claude Code, Codex and Cursor:
per-repo **projects** and user-level **homes** (`~/.claude`, `~/.codex`, `~/.cursor`), through one pipeline. It is a
generic engine. It knows harness layouts and **nothing about tickets, gates, docs or any process**: every project fact is
a free value under `vars:` in the manifest, read by templates as `.Vars`. This repository embeds only **examples**
(`examples/`, including the example bundle registry `examples/bundles.yaml`, and the runnable demo `examples/demo` with its
committed output). The user's own data — the `manifest.yaml`, their real templates and bundles in a `templates/` overlay,
`global/`, `agents/`, `projects/` — lives in a separate **config directory** (`--config DIR`, `$TRAFARET_CONFIG`, or the
nearest parent of the working directory holding a `manifest.yaml`). The rendered trees in target repos (`.claude/`,
`.agents/`, `.codex/agents/`, `.cursor/rules/`) and the managed files in homes are **build artifacts** — never edit them;
the next `trafaret install` overwrites your edit. Never re-create `.cursor/skills` / `.cursor/agents` in target repos —
that double-loads every skill in Cursor.

## Edit workflow (always)

1. Edit the source:
   - a shared skill/agent/rule → `<config>/templates/{skills,agents,rules}/…` in the config directory (Go
     text/template; read values as `.Vars.<key>`, see `docs/template-reference.md`). Logic shared by several templates
     goes in `<config>/templates/partials/*.tmpl`. A file there overrides the embedded example at the same path and new
     files are added; which items install together is declared in `<config>/templates/bundles.yaml`. In this repository,
     `examples/` holds examples only — change them only to keep the examples, the demo output and the docs accurate;
   - one project's skill, copied as it is → `<config>/projects/<name>/skills/<skill>/SKILL.md` (verbatim, no template
     pass);
   - a skill wanted in every home → `<config>/global/skills/<skill>/SKILL.md` (files ending `.tmpl` render, the rest is
     copied; installed into each `homes:` entry that omits `skills:` or names it). Never hand-edit those home copies; a
     skill that works in one harness only is listed under `skip:` of the other homes;
   - a role agent (a long-lived named session) → `<config>/agents/<role>/AGENT.md` (its boot doc, read by the session,
     never installed) and `<config>/agents/<role>/skills/<skill>/SKILL.md` (installed into the homes like global skills;
     names unique across `global/skills` and all roles);
   - a project's or home's values (branch, gates, tracker, docs, …) → `vars:` of its manifest entry, or `defaults.vars`
     / the top-level `vars` when they are shared.
2. `trafaret validate` — renders every declared item, so missing vars, template typos and frontmatter lint errors surface.
3. `trafaret diff <name>`, read the lines, then `trafaret install <name>` (or `--all`).
4. Commit the config (target repos normally git-ignore the rendered trees; see `docs/onboarding.md`). Developer-style
   messages, no AI attribution.

## Hard rules

- **The engine knows nothing about tickets or process.** Never add a typed project field, or engine code, for a project
  fact: it is a var, and logic over vars belongs in a template or a partial. The removed 0.2 fields (`tickets`, `run`,
  `gates`, `docs`, `stack`, `tools`, `vcs`, `e2e`, `targets`, `skillsDir`) are rejected loudly with a migration hint, not
  ignored.
- **Vars, not tokens.** Templates read `.Vars.<key>`; the data is `.Name .Kind .Harness .RepoRoot .TargetRoot .Vars`. Vars
  merge top-level `vars` < `defaults.vars` < the entry's `vars` (maps merge recursively, scalars and lists replace, `null`
  deletes). Optional values use `has` / `index`; `default` does not rescue an absent key. A change to the data fields, the
  funcs or the merge updates `docs/template-reference.md` and `cmd/skill.md` in the same commit.
- **One namespace, adapters for harnesses.** Skills share one namespace: the `templates/skills` overlay, `global/skills`,
  `agents/*/skills` and the embedded examples; a name defined twice among the config sources is an error and a config
  source shadows an embedded example. Files ending `.tmpl` render, every other file is copied byte for byte (only
  `projects/<name>/` is never rendered). Where an item lands per harness and scope is the adapter table in
  `internal/engine/adapters.go`. **Adding a harness is adding one adapter value there** (plus tests, and the adapter table
  in `README.md`, `docs/spec.md`, `docs/template-reference.md` and `cmd/skill.md`); the render, write, track and prune code
  does not change.
- **Golden files are the documentation.** `examples/demo` is a runnable config; its rendered output is committed under
  `examples/demo/out/<project-or-home>/<path as written>` and `TestGoldenDemo` fails when the sources and the output
  diverge. After an intended change to an example, the demo manifest or the engine, regenerate with
  `go test ./... -run Golden -update` and review the diff like code. README excerpts are copied from these files; keep them
  verbatim.
- **Rendering must stay byte-stable.** A change to a template or to the engine that alters what an existing manifest
  renders shows up as `update` lines in `trafaret diff`. Check `diff` against a real config before and after; a deliberate
  change is a deliberate commit.
- **Nothing private in this repository.** No personal names, hosts, IPs, workspace URLs, absolute home paths or emails in
  templates, docs, Go code or tests. A value a template needs comes from a manifest var; examples use neutral names
  (`acme`, `example.com`). The module import path may appear only in `README.md`, `CONTRIBUTING.md`, `SECURITY.md`,
  `go.mod`, `Makefile` and `.go` files. `TestNoPrivateStringsInSources` enforces the list over every file, docs included.
- **The bundle registry is data**, never Go code: `examples/bundles.yaml` for the embedded examples; a config's own
  `templates/bundles.yaml` shadows it (not merged). It is decoded strictly: an unknown field is an error. Adding a bundle
  field means extending the `Bundle` type in `internal/engine/bundles.go`, `loadBundles`' validation, `README.md`,
  `docs/spec.md`, `cmd/skill.md` and the header of `examples/bundles.yaml`.
- **Frontmatter is linted at render time** on every emitted `.md` / `.mdc`: it must parse as YAML; `SKILL.md` needs a
  non-empty `name` + `description`; unquoted values must not contain ` #` (YAML truncates there) or `: `.
- **Scope guards** are recommended for project-scoped skill templates: the description ends with
  `Applies only to the {{.Name}} repo.` (the shipped `ship` skill does). Same-named skills load side by side in
  multi-root workspaces and the clause is the only disambiguation Cursor gets. Home skills are global and carry none.
- Use `{{.TargetRoot}}` for any in-repo path to a skill or agent file — it renders as `.claude`, `.agents` or `.cursor`
  for the copy being rendered, and as the absolute harness dir in a home.
- Agent templates (`examples/agents/`) stay **pointers** to their skills — the skill is the single source of every
  checklist. Don't restate skill content in an agent file.
- **Track file names keep their historical names**: `.claude/ao-manifest.json` (per repo) and
  `<dir>/<skills|agents|rules>/.ao-manifest.json` (per home directory). Renaming them would orphan existing installs. Only
  tracked paths are ever pruned.
- **The CLI surface has one reference.** A change to a command, flag or message updates `cmd/skill.md` (embedded, printed
  by `trafaret skill`), the usage screen in `cmd/help.go` and `README.md` in the same commit; `cmd` tests fail when a
  command or flag is missing from `skill.md`. Flags are persistent on the root so they work before and after the command.
  `version` prints the build-stamped `cmd.version` to stdout and `version`/`skill`/`help` run without a config directory.
- Content doctrine: content lives where something maintains it. Process contracts belong in the templates; durable
  project knowledge belongs in that project's own docs.

## The operator site — edits here ARE edits to the site

`make site` serves a live bilingual manual at `http://127.0.0.1:4477` (`trafaret serve`). It re-renders from the config
directory on every refresh, so the live tables (sync status of every project and home, catalog, bundles) update
themselves — you never touch them.

What you DO maintain by hand is the static prose in `internal/site/strings.go`:

- When you change a **mechanism** (the vars merge, a template func, a bundle field, a CLI command, the lint, the overlay
  rules, an adapter), update the corresponding prose/diagram keys in **both** `siteEN` and `siteRU`. The page skeleton is
  `internal/site/page.go`.
- Key parity EN↔RU and skeleton↔maps is enforced by `go test` (`internal/site/site_test.go`) — `make check` fails on drift.
  Run it before committing site changes.
- Mermaid diagram labels must not contain `&lt;…&gt;` entities (htmlLabels swallows them as tags) — use `⟨…⟩`.

## Layout

| Path | What |
|---|---|
| `main.go` | calls `cmd.Execute()` |
| `cmd/` | cobra commands, one file per command; `root.go` holds the persistent flags, `version.go` the stamped `version`, `skill.go` + `skill.md` the embedded agent reference, `help.go` the usage screen |
| `internal/engine/` | `manifest.go` (manifest, removed-field rejection) and `vars.go` (the merge, `has`); `bundles.go` (the registry loader); `namespace.go` (the source namespace); `render.go` (data, funcs, render, lint); `adapters.go` (the harness adapter table); `install.go` and `prune.go` (write, track, prune); `hook.go` (the managed session hook); `ops.go` (what `install`/`diff`/`validate` run); `config.go` (config-directory resolution), `layered.go` (the templates overlay), `gate.go` (the `engine:` version gate) |
| `internal/site/` | the live operator site (`trafaret serve`): `data.go`, `page.go`, `strings.go`, `site.go`, vendored `assets/mermaid.min.js` |
| `examples/` | embedded example set — `embed.go` (`go:embed`), `bundles.yaml`, `skills/`, `agents/`, `rules/`, `partials/`; and `demo/` (not embedded): the runnable demo config `manifest.yaml` with its golden output in `out/` |
| `docs/spec.md` | design and rationale, the 0.2 to 0.3 migration |
| `docs/template-reference.md` | the template contract: data, funcs, vars merge, adapters, lint |
| `docs/onboarding.md` | onboarding a project and a home |
