# Onboarding a project into trafaret

Before adding a project to your config directory's `manifest.yaml` (the directory `trafaret root`
prints), decide which bundles it takes and confirm the project has what those bundles' templates read.
Which blocks that is depends on your own templates; a bundle's `requires:` in `templates/bundles.yaml`
(`tickets`, `db`) is what `trafaret validate` checks. Then add the block and install.

If you have no templates of your own yet, the embedded examples (`example`, `example-db`) render against
the blocks below and are a good way to see what each token produces. To write real ones, see "Bring your
own templates" in the README.

## Prerequisites checklist

Each item matters only if one of the project's bundles reads the matching block.

1. **A ticket board** (blocks `tickets`, `run`). Two databases (Tasks + Bugs, or one shared database in
   Work mode) with a state machine that maps onto `tickets.states` (for example `Backlog → To do →
   In progress → In review → To test`), a Priority enum, and a rich-text Description field. A ticket
   CLI profile (`tickets.cli --config <profile>`) that resolves to that workspace. A bundle with
   `requires: [tickets]` makes `validate` insist on it.
2. **A default-branch git policy** (block `vcs`). Default branch, whether work uses `git worktree`
   isolation, whether anything pushes, and the commit convention. If a template relies on a worktree,
   the project's build must be able to run from one.
3. **A per-package gate matrix** (block `gates`). Reproducible `build`/`test`/`lint` commands per
   package, and a stated lint-baseline policy ("new code clean, don't fix unrelated"). These become
   `gates.packages`.
4. **A docs taxonomy** (block `docs`). An orientation map (e.g. `docs/MAP.md`), per-domain context docs,
   normative policy docs, spec and plan directories, and optionally a publish + drift-guard pipeline
   (e.g. `mkdocs/` with `make docs-sync` / `make docs-check`).
5. **Normative policy docs** (block `stack`). Generic templates do NOT carry stack conventions (framework
   idioms, migration rules); they defer to `stack.conventionsDocs`. Encode your stack's rules there so a
   skill can point to them line-for-line.
6. **A cross-session memory CLI on PATH** with `recall` / `note` / `log` subcommands (name it in
   `tools.memory`, and what prose calls it in `tools.memoryName`; leave both unset if you have none).
7. **(Optional) an e2e harness** (`tools.e2e`) emitting a structured report and a file-bugs tool.
8. **(Optional) a read-only database CLI** (`tools.db`) for bundles with `requires: [db]` — the CLI, its
   profile, environment and tunnel command, and the project's own schema.

A project that takes only a bundle with `requires: [db]` (or no `requires`) needs none of the ticket
board.

## Add the manifest block

Copy a similar project block in your `manifest.yaml` and adjust. Minimum:

```yaml
  myproject:
    repoRoot: myproject            # relative to defaults.workspace
    bundles: [example]             # names from templates/bundles.yaml; start small
    tickets:
      cli: fibery
      config: <fibery-profile>
      space: "My Workspace"
      host: <workspace>.example.com
      boards: { tasks: Tasks, bugs: Bugs }
      states: { backlog: Backlog, todo: "To do", inProgress: "In progress", inReview: "In review", done: "To test" }
      priority: [Critical, High, Medium, Low]
      refScheme: { task: "Tn / Task-N", bug: "Bug-N" }
      dependsOn: "Depends on: <ref>"
    vcs:
      defaultBranch: main
      worktree: false
      push: false
      commitConvention: "developer-style message, no AI attribution"
      gitlabPath: gitlab.com/<group>/myproject
    gates:
      packages:
        - { name: backend, dir: backend, cmd: "make build && make test && make lint" }
      lintBaseline: "new code clean; do not fix unrelated lint"
    docs:
      map: docs/MAP.md
      contextDir: docs/context
      sourceDir: docs/source
      architect: docs/ARCHITECT.md
      specDir: docs/specs
      planDir: docs/plans
      syncCmd: "cd mkdocs && make docs-sync"
      checkCmd: "cd mkdocs && make docs-check"
      mandatorySections: [Purpose, Interfaces, Invariants]
    stack:
      envPrefix: MYPROJECT_
      conventionsDocs: [docs/source/dev-policy.md]
      subScopeAgents: [backend/AGENTS.md]
    tools:
      memory: mem
      e2e: { runCmd: "make e2e", report: e2e/reports/failures.json, fileBugs: "bash e2e/scripts/file-bugs.sh" }
```

## Install + verify

```bash
trafaret validate                # render every template, catch token typos / missing fields
trafaret diff myproject          # see what would be written
trafaret install myproject       # write Claude, Cursor, and Codex artifacts
```

Then open a rendered file (for the `example` bundle, `myproject/.claude/skills/example-tickets/SKILL.md`)
and confirm the board names, states and config profile match the live workspace before relying on it.

Codex reads the parallel skills from `.agents/skills/` and native custom agents from
`.codex/agents/`; skills may reference the matching Markdown contracts in `.agents/agents/`. All are
generated from the same sources; never edit the rendered copies.

## What to track

**Nothing `trafaret` renders.** Your config is the source of truth and `trafaret install` is a precondition
for working in a project, so a committed copy of the output is only ever a second, lagging one. The
symptom was concrete: one template change landed as the same diff in ten repositories and waited to
be committed ten times.

Every project ignores the rendered trees:

```gitignore
.claude/agents/*
.claude/skills/*
!.claude/skills/README.md
.claude/ao-manifest.json
.cursor/rules/<rendered-rule>.mdc     # one line per rule your bundles render
.agents/
.codex/
```

What stays tracked is what a human wrote: `.claude/settings.json` (hooks, permissions — trafaret only
adds and removes its own single SessionStart entry there, for a bundle that sets `sessionContext`), any
catalog `README.md`, and Cursor rules trafaret does not render. Note the shape of the first two lines:
`dir/*` rather than `dir/`, so a hand-authored file inside a rendered directory can still be re-included
— git cannot re-include anything under a directory excluded outright.

`.claude/ao-manifest.json` is ignored along with the rest. It is trafaret's own ledger of what it
rendered, read by `pruneStale` to delete artifacts a later render dropped — build state, not a
declaration of what the repo contains.

This assumes every environment that uses the rendered files has `trafaret` and your config directory.
If one ever has to work from a bare clone (a cloud session, someone else's machine), that environment
needs a bootstrap step, or the tracking rule flips back for `.claude/`.

## Cursor

Cursor reads `.claude/skills` and `.claude/agents` natively, so the `cursor` target contributes only
`.cursor/rules/`. `.cursor/` is often gitignored as local IDE state; if a project does track rendered
artifacts, whitelist just the rules directory:

```gitignore
.cursor/*
!.cursor/rules/
```

Never create `.cursor/skills` or `.cursor/agents` by hand or by copying: Cursor does not dedupe
same-named skills, so a second copy loads everything twice.
