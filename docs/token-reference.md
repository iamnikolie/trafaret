# Token reference — the template contract

Template files (`*.tmpl`) are Go `text/template` files. The binary embeds a few examples (the
`examples/` directory of the repository); a config directory adds its own, or overrides an embedded one, at
`<config>/templates/<same path>`, and groups them into bundles in `<config>/templates/bundles.yaml`
(see "Bundle registry" at the end). The renderer (`trafaret`) loads `manifest.yaml`, resolves one
**Project** struct per project, and executes every template with that project as the root dot (`.`).
This file is the **authoritative list of template expressions**. When you tokenize a source file,
replace each project-specific literal with the EXACT expression from the substitution table below —
copy them verbatim, do not invent new field paths.

Template functions: `dash` joins words with hyphens (`{{.Tools.MemoryName | dash}}`: `team memory` →
`team-memory`).

## Manifest top level

| Key | Meaning |
|---|---|
| `engine` | optional version constraint on the trafaret binary: `">=0.2.0"`, `"0.2.1"`, or a comma-separated range `">=0.2.0, <1.0.0"`. A binary that does not satisfy it exits with an error; absent = no check; an unstamped `dev` build always passes; a constraint that cannot be parsed is an error |
| `defaults.workspace` | absolute path that relative `repoRoot` values resolve against |
| `defaults.targets` | `[claude, cursor, codex]` — the targets a project gets unless it sets its own |
| `defaults.tickets.extraFields` | inherited by every project that sets no `tickets.extraFields` |
| `defaults.tools.memoryName` | inherited by every project that sets no `tools.memoryName` |
| `projects.<name>` | one block per project; the fields below, plus `bundles` (names from `templates/bundles.yaml` to install), `disabled` (bundles switched off; install prunes their artifacts) and `targets` (overrides `defaults.targets`) |
| `homes.<name>` | a per-user skills directory (`skillsDir`, optional `skills` subset, optional `skip`) that receives `global/skills/` and `agents/*/skills/` verbatim |

If a source file contains a literal `{{` or `}}` that is NOT a token (rare), escape it as
`{{"{{"}}` / `{{"}}"}}` and flag it in your report.

## Project struct → template expressions

### Identity
| Expression | Example value | Meaning |
|---|---|---|
| `{{.Name}}` | `acme-app` | project key in the manifest |
| `{{.RepoRoot}}` | `/work/acme-app` | absolute repo root (resolved) |
| `{{.TargetRoot}}` | `.claude` (skills/agents), `.cursor` (rules) | the config tree the rendered file lands in — use it for any in-repo path to a skill/agent file, never hardcode `.claude/` |

Every skill description ends with the scope clause `Applies only to the {{.Name}} repo at {{.RepoRoot}}.`
and every skill body opens with a `Project: {{.Name}}` guard line — this is what disambiguates same-named
skills when several projects are open in one workspace (Cursor loads them all with no dir-qualification).

**Frontmatter is linted at render time**: it must parse as YAML, `SKILL.md` needs non-empty
`name`+`description`, and an unquoted value containing ` #` is rejected (YAML silently truncates there).
Keep `: ` and ` #` out of unquoted frontmatter values — including via token values.

**Per-project verbatim sources**: files under `projects/<name>/{skills,agents}/` are copied as-is into
that project's `.claude/` tree on install (no template pass — they may contain `{{`).
Use them for project-specific skills that don't generalize into `templates/`.

**One tree for both harnesses**: skills and agents install only into `.claude/` — Cursor reads
`.claude/skills` and `.claude/agents` natively (cross-agent compatibility per the Cursor docs), and a
parallel `.cursor/` copy would double-load every skill (Cursor doesn't dedupe same-named skills).
The cursor target contributes only `.cursor/rules/`.

### Tickets (the ticket/board protocol)
| Expression | Example value |
|---|---|
| `{{.Tickets.CLI}}` | `fibery` |
| `{{.Tickets.Config}}` | `acme` (the `fibery --config <X>` profile) |
| `{{.Tickets.Space}}` | `Acme development` |
| `{{.Tickets.Host}}` | `acme.example.com` |
| `{{.Tickets.Boards.Tasks}}` | `Tasks` |
| `{{.Tickets.Boards.Bugs}}` | `Bugs` |
| `{{.Tickets.TasksDB}}` | `Acme development/Tasks` (computed) |
| `{{.Tickets.BugsDB}}` | `Acme development/Bugs` (computed) |
| `{{.Tickets.States.Backlog}}` | `Backlog` |
| `{{.Tickets.States.Todo}}` | `To do` |
| `{{.Tickets.States.InProgress}}` | `In progress` |
| `{{.Tickets.States.InReview}}` | `In review` |
| `{{.Tickets.States.Done}}` | `To test` (the state an item reaches when the work is finished and handed on) |
| `{{.Tickets.PriorityChain}}` | `Critical → High → Medium → Low` (computed) |
| `{{range .Tickets.Priority}}` | iterate `Critical`,`High`,`Medium`,`Low` |
| `{{.Tickets.RefScheme.Task}}` | `Tn / Task-N` |
| `{{.Tickets.RefScheme.Bug}}` | `Bug-N` |
| `{{.Tickets.DependsOn}}` | `Depends on: <ref>` |
| `{{.Tickets.ConfigDir}}` | `~/.fibery/acme` (computed) |
| `{{.Tickets.DB}}` / `{{.Tickets.Project}}` | Work mode only: `Development/Work` / `Widgets` — the shared tasks+bugs database and this project's `Project` value. With `db:` set, `TasksDB` and `BugsDB` both render as it |
| `{{if .Tickets.Work}}` | true when `tickets.db` is set |
| `{{.Tickets.ExtraFields}}` | free text (`tickets.extraFields`, else `defaults.tickets.extraFields`) listed after the standard Work-mode fields in a ticket-protocol skill — workspace-specific fields such as ``` `Development/Seat` (the role holding it) ```. Empty = none |
| `{{.Tickets.ProjectFilter}}` | ` --filter "Development/Project=Widgets"` (leading space; empty outside Work mode) — append to `list` / `search` / `count` |
| `{{.Tickets.TaskFilter}}` / `{{.Tickets.BugFilter}}` | ProjectFilter plus ` --filter "Development/Type=Task"` / `Bug` |
| `{{.Tickets.TaskSet}}` / `{{.Tickets.BugSet}}` | ` "Development/Project=Widgets" "Development/Type=Task"` / `Bug` — append to `create` |
| `{{.Tickets.ProjectWhere}}` / `{{.Tickets.ProjectParams}}` | raw FQL: a `"q/where": […],` line and its ` --params '{"$project":…}'`; wrap the where line in `{{if .Tickets.Work}}` |

### VCS
| Expression | Example value |
|---|---|
| `{{.VCS.DefaultBranch}}` | `main` |
| `{{.VCS.Worktree}}` | `false` (bool) |
| `{{.VCS.Push}}` | `false` (bool) |
| `{{.VCS.CommitConvention}}` | `developer-style message, no AI attribution` |
| `{{.VCS.GitlabPath}}` | `gitlab.com/acme/acme-app` |

Conditional prose on branch policy: `{{if .VCS.Worktree}}…{{else}}share the same {{.VCS.DefaultBranch}} working tree — do NOT use worktree isolation{{end}}`.
Push prose: `{{if not .VCS.Push}}**no push**{{end}}`.

### Gates (per-package gate matrix)
| Expression | Meaning |
|---|---|
| `{{range .Gates.Packages}}` … `{{.Name}}` `{{.Dir}}` `{{.Cmd}}` … `{{end}}` | iterate packages |
| `{{.Gates.LintBaseline}}` | lint-baseline policy string |

Render a gate list like:
```
{{range .Gates.Packages}}- `{{.Dir}}`: `{{.Cmd}}`
{{end}}
```

### Docs taxonomy
| Expression | Example value |
|---|---|
| `{{.Docs.Map}}` | `docs/MAP.md` |
| `{{.Docs.ContextDir}}` | `docs/context` |
| `{{.Docs.SourceDir}}` | `docs/source` |
| `{{.Docs.Architect}}` | `docs/ARCHITECT.md` |
| `{{.Docs.SpecDir}}` | `docs/specs` |
| `{{.Docs.PlanDir}}` | `docs/plans` |
| `{{.Docs.SyncCmd}}` | `cd mkdocs && make docs-sync` |
| `{{.Docs.CheckCmd}}` | `cd mkdocs && make docs-check` |
| `{{range .Docs.CommitPaths}}` | `docs/`, `mkdocs/` — paths a docs-maintaining skill stages on commit (defaults to the dir of `Map` if unset) |
| `{{range .Docs.MandatorySections}}` | the section names every context doc must carry (a list you define) |

A per-domain context path: `` `{{.Docs.ContextDir}}/<domain>.md` ``.

### Stack
| Expression | Example value |
|---|---|
| `{{.Stack.EnvPrefix}}` | `ACME_` |
| `{{range .Stack.ConventionsDocs}}` | `docs/source/dev-policy.md`, `docs/source/frontend-dev-policy.md` |
| `{{range .Stack.SubScopeAgents}}` | `backend/AGENTS.md`, `frontend/AGENTS.md` |

**Important generalization rule:** stack-specific coding conventions (Go `dig.As`, `gorm`, FSD,
migration-timestamp rules, the `ACME_` examples in prose) must NOT be hardcoded in the
template. Replace them with a reference to the project's own policy docs:
> Follow the conventions in {{range .Stack.ConventionsDocs}}`{{.}}` {{end}}— they are normative.

Keep only the *generic* engineering stance (TDD, scope discipline, "prove each criterion").

### Tools
| Expression | Example value |
|---|---|
| `{{.Tools.Memory}}` | `mem` (a cross-session memory CLI; `{{.Tools.Memory}} recall`/`note`/`log`) |
| `{{.Tools.MemoryName}}` | `team memory` — what prose calls that system (`tools.memoryName`, else `defaults.tools.memoryName`, else `memory`); `{{.Tools.MemoryName \| dash}}` hyphenates it |
| `{{.Tools.E2E.RunCmd}}` | `make e2e` |
| `{{.Tools.E2E.Report}}` | `e2e/reports/failures.json` |
| `{{.Tools.E2E.FileBugs}}` | `bash e2e/scripts/file-bugs.sh` |
| `{{.Tools.DB.CLI}}` | `dbq` (a read-only database CLI) — empty means the project has no `db` bundle; templates gate DB lines on it |
| `{{.Tools.DB.Config}}` | `acme` (the CLI's `--config <profile>`) |
| `{{.Tools.DB.Env}}` | `dev` |
| `{{.Tools.DB.Schema}}` | `acme_app` — the project's own database; empty for a repo that spans every app |
| `{{.Tools.DB.Tunnel}}` | `lsof … \|\| ssh -N -f -L 13306:localhost:3306 db-host` — idempotent reachability step |
| `{{.Tools.DB.Cmd}}` | `dbq --config acme --env dev` (computed prefix of every call) |

### Run defaults (per-project answers for a run-style skill)

The `run:` block holds this project's **default answers** to the questions a run-style skill asks at
the start of a run; the operator overrides any of them interactively. `profile` expands into
`ReviewDepth`, `MaxRounds` and `OverlapOn` — set `maxRounds` / `overlap` explicitly only to deviate from
the profile.

| Expression | Example value |
|---|---|
| `{{.Run.Profile}}` | `balanced` (`careful` · `balanced` · `fast`) |
| `{{.Run.ReviewDepth}}` | computed — `per-item` (careful, balanced) or `split` (fast) |
| `{{.Run.MaxRounds}}` | computed from the profile unless `maxRounds` is set (3 · 3 · 2) |
| `{{.Run.OverlapOn}}` | computed from the profile unless `overlap` is set (bool; careful `false`) |
| `{{.Run.Order}}` | `priority` or `cheap-first` |
| `{{.Run.AskAtStart}}` | `true` unless `interactive: false` (bool) |

Conditional prose on the plan: `{{if .Run.AskAtStart}}ask the operator…{{else}}run the defaults…{{end}}`,
`overlap={{if .Run.OverlapOn}}on{{else}}off{{end}}`.

Every project gets a resolved `Run` even without a `run:` block — the fallback is `balanced` /
`priority` / interactive. For a project with a bundle that `requires: [tickets]`, `trafaret validate`
rejects an unknown profile, an order outside `priority|cheap-first`, and `maxRounds` outside 1..5.

## Substitution table (literal → expression)

When you tokenize a hand-written source, replace each project-specific literal (examples from a
fictional `acme-app`) with the expression:

| Literal in source | Replace with |
|---|---|
| `--config acme` | `--config {{.Tickets.Config}}` |
| `acme.example.com` | `{{.Tickets.Host}}` |
| `Acme development` | `{{.Tickets.Space}}` |
| `Acme development/Tasks` | `{{.Tickets.TasksDB}}` |
| `Acme development/Bugs` | `{{.Tickets.BugsDB}}` |
| `~/.fibery/acme` | `{{.Tickets.ConfigDir}}` |
| `FIBERY_CONFIG=acme` | `FIBERY_CONFIG={{.Tickets.Config}}` |
| `To do` (state) | `{{.Tickets.States.Todo}}` |
| `In progress` (state) | `{{.Tickets.States.InProgress}}` |
| `In review` (state) | `{{.Tickets.States.InReview}}` |
| `To test` (state) | `{{.Tickets.States.Done}}` |
| `Backlog` (state) | `{{.Tickets.States.Backlog}}` |
| `Critical → High → Medium → Low` | `{{.Tickets.PriorityChain}}` |
| `Tn`/`Task-N` (ref scheme) | `{{.Tickets.RefScheme.Task}}` |
| `Bug-N` (ref scheme) | `{{.Tickets.RefScheme.Bug}}` |
| `Depends on:` (protocol) | `{{.Tickets.DependsOn}}` (or keep literal "Depends on:" where it reads as prose) |
| `/work/acme-app` | `{{.RepoRoot}}` |
| `main` (the branch) | `{{.VCS.DefaultBranch}}` |
| `no AI attribution` commit rule | `{{.VCS.CommitConvention}}` |
| `gitlab.com/acme/acme-app` | `{{.VCS.GitlabPath}}` |
| the backend/frontend gate commands | `{{range .Gates.Packages}}…{{end}}` |
| `docs/MAP.md` | `{{.Docs.Map}}` |
| `docs/context/<domain>.md` | `{{.Docs.ContextDir}}/<domain>.md` |
| `docs/source/dev-policy.md` (+ frontend) | `{{range .Stack.ConventionsDocs}}…{{end}}` |
| `docs/ARCHITECT.md` | `{{.Docs.Architect}}` |
| `docs/specs` | `{{.Docs.SpecDir}}` |
| `docs/plans` | `{{.Docs.PlanDir}}` |
| `cd mkdocs && make docs-sync` | `{{.Docs.SyncCmd}}` |
| `cd mkdocs && make docs-check` | `{{.Docs.CheckCmd}}` |
| `ACME_` | `{{.Stack.EnvPrefix}}` |
| `mem recall`/`mem note`/`mem log` | `{{.Tools.Memory}} recall` / `… note` / `… log` |
| `e2e/reports/failures.json` | `{{.Tools.E2E.Report}}` |
| `bash e2e/scripts/file-bugs.sh` | `{{.Tools.E2E.FileBugs}}` |
| `make e2e` | `{{.Tools.E2E.RunCmd}}` |

Anything project-specific NOT covered here (a NATS subject, a concrete env-key name, an
item-specific review heuristic like "refuse catalog-absent prices") is **stack content that
belongs in the project's own policy/context docs, not in the generic template** — drop it from the
template and rely on `{{range .Stack.ConventionsDocs}}`.

## Bundle registry (`templates/bundles.yaml`)

A bundle is a named set of templates; the registry is a YAML map from bundle name to its entries. It is
read from the template set, so a config's own `templates/bundles.yaml` **shadows** the embedded one (it
is not merged).

| Field | Meaning |
|---|---|
| `skills` | directories under `templates/skills/`; every file directly in them renders with the project's tokens, the `.tmpl` suffix dropped |
| `agents` | `templates/agents/<name>.md.tmpl` (Codex also gets a native TOML form) |
| `cursorRules` | `templates/cursor/rules/<name>.mdc.tmpl` |
| `requires` | subset of `tickets`, `db` — the manifest blocks the templates read; `validate` checks them (`tickets`: `tickets.config`, `tickets.space`, Work-mode pairing and the `run` block; `db`: `tools.db.cli`, `.config`, `.tunnel`) |
| `sessionContext` | a relative file under `.claude/skills/` printed by a SessionStart hook trafaret manages in the project's `.claude/settings.json`; at most one active bundle per project |
