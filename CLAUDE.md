# trafaret

Read `AGENTS.md` — it is the single source of the working agreements for this repo (edit workflow,
frontmatter lint rules, scope-guard conventions, the no-private-data rule, the data-driven bundle
registry, the CLI-surface contract: `cmd/skill.md` and the usage screen follow every command/flag change,
and the operator-site contract: mechanism changes require updating `internal/site/strings.go` in both
languages; `go test` enforces key parity).

Quick reminders: the templates here (`examples/`) are only examples, embedded in the binary — rebuild
(`make build`) after editing them. A user's `manifest.yaml`, real templates and `bundles.yaml` (the `templates/`
overlay), `global/`, `agents/`, `projects/` live in their config directory, never in this repo. Never
edit rendered `.claude/`/`.cursor/`/`.agents/`/`.codex/` trees in target repos; run
`trafaret validate && trafaret install --all` instead. Developer-style commit messages, no AI
attribution.
