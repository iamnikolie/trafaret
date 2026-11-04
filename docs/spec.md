# trafaret — design & rationale

## Problem

Agent skills, subagents and rules are valuable, but written once for one project they are hardcoded to
that project's ticket board, docs paths, gate matrix and stack conventions. Copy-pasted into other repos
they drift: the ticket cheat-sheet differs by workspace name only, yet each copy ages separately, and
only the repo that happened to get the files has them at all. There is no shared layer, and each harness
(Claude Code, Cursor, Codex) wants the files in its own place and format.

## Goal

One tokenized source of truth for a library of skills, agents and rules, parameterized per project and
installed by a small CLI into each project's native Claude, Cursor and Codex locations. Kill the
duplication.

trafaret is the **engine**: it renders, lints, writes and prunes. It does not ship a library. Its binary
embeds a handful of examples that show the template format and every token family; the real templates
belong to whoever runs it and live in their own config directory.

## Approach (chosen)

**Templated install.** Generic, tokenized Go `text/template` sources plus per-project token values in a
`manifest.yaml`; `trafaret install <project>` renders concrete, self-contained artifacts into the
project. No runtime indirection (an unattended agent must not depend on a model remembering to read a
profile mid-run); the rendered `.claude/` and `.agents/` files are fully concrete.

Alternatives rejected:
- _Runtime profile_ (generic skills + a profile doc read at runtime) — reliability risk for an
  unattended agent, and a copy mechanism is needed regardless (skills must physically live in
  `.claude/skills/`; symlinks break under worktrees).
- _A ticket-system registry/showcase_ — pays off for broad team distribution; for a handful of repos it
  is pure overhead next to a manifest.
- _Config-as-code bundle_ (settings, statusline, memory sync in the same tool) — different lifecycle and
  different owners; keeping this tool to skills/agents/rules keeps it small.

## Architecture

- **Engine and config are separate.** The binary embeds the example template set (`go:embed`, served
  through `io/fs`); everything that belongs to the user lives in a **config directory** — the directory
  holding `manifest.yaml`, plus optional `global/`, `agents/`, `projects/` and a `templates/` overlay.
  The directory is chosen by `--config`, then `$TRAFARET_CONFIG`, then by walking up from the working
  directory. One binary can serve any number of independent configs (a personal one, a team one), each
  with its own templates.
- **Templates overlay.** `<config>/templates/<path>` replaces the embedded file with the same path and may
  add files; directories merge. A config's real templates are simply files in this overlay, so adding or
  changing one never means forking the engine.
- **Data-driven bundles.** A bundle is a named set of templates. The registry is data —
  `templates/bundles.yaml`, a map from bundle name to `skills`, `agents`, `cursorRules`, `requires` and
  `sessionContext` — loaded at startup into the engine. It goes through the same overlay as any template,
  so a config's own `bundles.yaml` **shadows** the embedded one: it is not merged, and a config defines
  exactly the bundles it lists. Nothing in the engine knows a bundle by name.
  - `requires` (`tickets`, `db`) tells `validate` which manifest blocks a bundle's templates read, so a
    project is only held to the blocks its bundles need (a database-only bundle needs no ticket board).
  - `sessionContext` names a file under `.claude/skills/` that a SessionStart hook prints at every session
    start. trafaret owns exactly one such entry in the project's `.claude/settings.json`, merges it
    additively, preserves every other key, and removes it when its bundle is disabled. At most one active
    bundle per project may set it.
- **Engine gate.** An optional `engine:` constraint in the manifest makes a config refuse to run on a
  binary that is too old for the tokens it uses.
- `templates/` (embedded examples) — `bundles.yaml`, `skills/<name>/` (a `SKILL.md.tmpl` plus any other
  files, all rendered), `agents/<name>.md.tmpl` (subagent contracts), `cursor/rules/<name>.mdc.tmpl`
  (Cursor rules).
- Codex projection — skills render to `.agents/skills/`; Markdown agent contracts render to
  `.agents/agents/` for skill references and convert to native `.codex/agents/*.toml` with name,
  description, instructions, model tier, and sandbox mode.
- `manifest.yaml` — `defaults` (workspace, targets, shared prose fragments) + `projects.<name>` with the
  full token block + `homes.<name>` for per-user skills directories. YAML anchors keep shared shapes DRY.
- The engine loads the manifest, resolves each Project (computing `TasksDB`, `BugsDB`, `PriorityChain`,
  `ConfigDir`, absolute `RepoRoot`, the run profile), renders the bundles' templates with
  `missingkey=error` (so a token typo fails loudly), lints the rendered frontmatter, and writes artifacts
  sha256-idempotently, pruning what a later render no longer produces. Commands: `install`, `diff`,
  `list`, `validate`, `root`, `serve`, `version`.
- `docs/token-reference.md` — the contract: every `{{.Token}}` expression and the literal→expression
  substitution table used to tokenize a hand-written source.

## The generalization principle

When a hand-written skill is turned into a template, most of it is generic engineering stance; the
coupling concentrates in three substitutable surfaces: (1) the ticket protocol, (2) the docs taxonomy,
(3) the per-package gate matrix and stack conventions. Tokens cover (1) and (2). For (3), stack-specific
coding rules (framework idioms, migration ordering, env-prefix examples) are **pulled out of the
templates** and replaced with a reference to the project's own `stack.conventionsDocs`. A generic
template keeps only the transferable stance.

Anything private to a deployment — hosts, workspace URLs, account names, product names of in-house
tools — belongs in a manifest token or in the config directory, never in an embedded template. This
repository is public, and a test enforces it.

## Out of scope

A ticket-system registry, memory sync, settings/statusline/personal-CLAUDE.md bundling, and skill
wrappers for individual CLIs beyond what a config's `global/skills/` carries. The engine also ships no
process library of its own: skills, agents and bundles are the user's to write.
