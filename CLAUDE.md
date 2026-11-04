# trafaret

Read `AGENTS.md` — it is the single source of the working agreements for this repo (edit workflow; the engine knows
nothing about tickets or process, so project facts are `vars`; the namespace and the harness adapter table; golden files
under `examples/demo/out` are the documentation; the frontmatter lint; no private data; the data-driven, strict bundle
registry; track file names; the CLI-surface contract: `cmd/skill.md`, the usage screen and `README.md` follow every
command/flag change; and the operator-site contract: mechanism changes require updating `internal/site/strings.go` in both
languages, `go test` enforces key parity).

Quick reminders: the templates here (`examples/`) are only examples, embedded in the binary — rebuild (`make build`) after
editing them, and regenerate the demo output with `go test ./... -run Golden -update`, reviewing the diff. A user's
`manifest.yaml`, real templates and `bundles.yaml` (the `templates/` overlay), `global/`, `agents/`, `projects/` live in
their config directory, never in this repo. Never edit rendered `.claude/`/`.agents/`/`.codex/`/`.cursor/` trees in target
repos or the managed files in a home; run `trafaret validate && trafaret install --all` instead. Developer-style commit
messages, no AI attribution.
