# trafaret — agent notes

`trafaret` is a Go CLI that renders a library of agent skills, subagents and rules into projects. It is
a generic engine: this repository embeds only **examples** (`templates/`, including the example bundle
registry `templates/bundles.yaml`). The user's own data — the `manifest.yaml`, their real templates and
bundles in a `templates/` overlay, `global/`, `agents/`, `projects/` — lives in a separate **config
directory** (`--config DIR`, `$TRAFARET_CONFIG`, or the nearest parent of the working directory holding a
`manifest.yaml`). The rendered trees in target repos (`.claude/`, `.cursor/rules/`, `.agents/`,
`.codex/agents/`) are **build artifacts** — never edit them in the target repos; the next
`trafaret install` overwrites your edit. Never re-create `.cursor/skills` / `.cursor/agents` in target
repos — that double-loads every skill in Cursor.

## Edit workflow (always)

1. Edit the source:
   - a shared skill/agent/rule → `<config>/templates/…` in the config directory (Go text/template,
     tokenized — see `docs/token-reference.md` for every `{{.Token}}`). A file there overrides the
     embedded example at the same path and new files are added; which templates install together is
     declared in `<config>/templates/bundles.yaml`. In this repository, `templates/` holds examples only —
     change them only to keep the examples and the docs accurate;
   - one project's skill → `<config>/projects/<name>/skills/<skill>/SKILL.md` (verbatim, no tokens);
   - a skill for a CLI wanted in every repo → `<config>/global/skills/<skill>/SKILL.md` (verbatim;
     installed into each `homes:` entry of the manifest). Never hand-edit those home copies; no scope
     guard (they are global); a skill that works in one harness only is listed under `skip:` of the
     other homes;
   - a role agent (a long-lived named session) → `<config>/agents/<role>/AGENT.md` (its boot doc, read
     by the session, never installed) and `<config>/agents/<role>/skills/<skill>/SKILL.md` (installed
     into the homes like global skills; names unique across `global/skills` and all roles; no scope
     guard);
   - a project's boards/branch/gates/docs → its block in `manifest.yaml`.
2. `trafaret validate` — catches token typos and frontmatter lint errors.
3. `trafaret install --all` (or `trafaret install <project>`).
4. Commit the config and the touched rendered repos. Developer-style messages, no AI attribution.

## Hard rules

- **Rendering must stay byte-stable.** A change to a template or to the engine that alters what an
  existing manifest renders shows up as `update` lines in `trafaret diff`. Check `diff` against a real
  config before and after; a deliberate change is a deliberate commit.
- **Nothing private in this repository.** No personal names, hosts, IPs, workspace URLs, absolute home
  paths or emails in templates, docs, Go code or tests. A value a template needs comes from a manifest
  token (add it to `docs/token-reference.md`); examples use neutral names (`acme`, `example.com`).
  `TestNoPrivateStringsInSources` enforces the list.
- **The bundle registry is data.** Bundles are defined in `templates/bundles.yaml`, never in Go code. A
  config's own `templates/bundles.yaml` shadows the embedded one (it is not merged), so a config defines
  exactly its own bundles. Adding a bundle field means extending the `bundle` type in `main.go`,
  `loadBundles`' validation, `README.md` and the header of `templates/bundles.yaml`.
- **Frontmatter is linted at render time**: must parse as YAML; `SKILL.md` needs non-empty
  `name` + `description`; unquoted values must not contain ` #` or `: ` (YAML truncates/breaks
  there).
- **Scope guards are mandatory** in every skill template shipped here, and recommended for a config's
  own: the description ends with `Applies only to the <name> repo at <abs path>.` and the body opens
  with a `Project:` guard line. Same-named skills load side-by-side in multi-root workspaces; the
  guards are the only disambiguation Cursor gets.
- Use `{{.TargetRoot}}` for any in-repo path to a skill/agent file — it renders as `.claude` or
  `.agents` for the active harness.
- Agent templates (`templates/agents/`) stay **pointers** to their skills — the skill is the
  single source of every checklist. Don't restate skill content in an agent file.
- The tracking files `.claude/ao-manifest.json` (per repo) and `<skillsDir>/.ao-manifest.json` (per
  home) keep their historical names; renaming them would orphan existing installs.
- Content doctrine: content lives where something maintains it. Process contracts belong in the
  templates; durable project knowledge belongs in that project's own docs.

## The operator site — edits here ARE edits to the site

`make site` serves a live bilingual manual at `http://127.0.0.1:4477` (`trafaret serve`). It re-renders
from the config directory on every refresh, so the live tables (project sync status, catalog, bundles)
update themselves — you never touch them.

What you DO maintain by hand is the static prose in `site_strings.go`:

- When you change a **mechanism** (a token, a bundle field, a CLI command, the lint, the overlay rules),
  update the corresponding prose/diagram keys in **both** `siteEN` and `siteRU`. The page skeleton is
  `site_page.go`.
- Key parity EN↔RU and skeleton↔maps is enforced by `go test` (`site_test.go`) — `make check`
  fails on drift. Run it before committing site changes.
- Mermaid diagram labels must not contain `&lt;…&gt;` entities (htmlLabels swallows them as tags) —
  use `⟨…⟩`.

## Layout

| Path | What |
|---|---|
| `main.go` | the CLI: load manifest → render per target → lint → write (sha256-idempotent); `loadBundles` reads the registry |
| `config.go` | config-directory resolution, the embedded templates + overlay (`io/fs`), the `engine:` version gate |
| `site.go`, `site_page.go`, `site_strings.go`, `site_test.go` | the live operator site (`trafaret serve`) |
| `templates/bundles.yaml` | the embedded bundle registry (examples `example`, `example-db`) |
| `templates/skills/`, `templates/agents/`, `templates/cursor/rules/` | embedded example templates — `go:embed` |
| `docs/token-reference.md` | the template contract |
| `docs/onboarding.md`, `docs/spec.md` | onboarding + design rationale |
