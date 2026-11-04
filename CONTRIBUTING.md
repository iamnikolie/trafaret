# Contributing

Thanks for taking a look. Bug reports and pull requests are welcome, and so is
a plain question in an issue.

## Reporting a bug

Include the output of `trafaret version`, the command you ran, and what you
expected. When the problem is a render, the output of `trafaret validate` and
the template or manifest block involved is the most useful single thing.
**Redact hostnames, workspace names and paths** — this is a public tracker and
those describe someone's setup.

## The rules that hold the project together

- **Rendering is byte-stable.** A change to the engine or to an example
  template that alters what an existing manifest renders shows up as `update`
  lines in `trafaret diff`. Check `diff` against a real config before and
  after; a deliberate change is a deliberate commit. The demo's committed
  output, `examples/demo/out`, is the documentation of what the examples
  render to: `go test ./...` fails until you regenerate it with
  `go test ./... -run Golden -update`, and the diff of `out/` is reviewed like
  code.
- **The engine is generic.** It knows harness layouts and nothing about
  tickets or any process. Nothing in the repository may name a person, a host,
  an address, a workspace URL, an absolute home path or an email. A value a
  template needs comes from a manifest var (`vars:`, read as `.Vars`; the
  template contract is in `docs/template-reference.md`); examples use neutral
  names (`acme`, `example.com`). `TestNoPrivateStringsInSources` scans the
  whole tree.
- **The bundle registry is data.** Bundles live in `examples/bundles.yaml` (the
  embedded examples) or in a config's own `templates/bundles.yaml`, never in Go
  code. A config's `bundles.yaml` shadows the embedded one; it is not merged.
- **Frontmatter is linted at render time.** It must parse as YAML, `SKILL.md`
  needs a non-empty `name` and `description`, and no unquoted value may contain
  ` #`.
- **Only what trafaret wrote is ever pruned.** Prune works from the track
  files (`.claude/ao-manifest.json` in a repo, `.ao-manifest.json` in each of
  `skills/`, `agents/` and `rules/` of a home), never from a directory scan.
  Their historical names stay: renaming them would orphan existing installs.

## Layout

| Path | What |
|---|---|
| `main.go` | calls `cmd.Execute()` |
| `cmd/` | the cobra commands, one file per command; `skill.md` is embedded and printed by `trafaret skill` |
| `internal/engine/` | manifest and vars merge, bundle registry, source namespace, render, harness adapters, install, prune, session hook, config-directory resolution, layered templates, the `engine:` version gate |
| `internal/site/` | the operator site behind `trafaret serve`: data collection, page skeleton, EN/RU strings, vendored mermaid |
| `examples/` | the embedded example templates and `bundles.yaml` (`go:embed`), and `demo/`, a runnable config with its golden output in `demo/out` |
| `docs/` | the template reference, onboarding guide and design notes |

## Pull requests

```bash
make fmt
make vet
make test
```

CI runs gofmt, `go vet` and `go test -race` on Linux and macOS.

House rules:

- **One concern per PR.**
- **Tests stay hermetic.** `go test ./...` must pass with no network and no
  config outside the temp directories the tests create.
- **Data to stdout, everything else to stderr.** `version`, `list`, `root`,
  the `validate` report and `--json` output are data; errors are not.
- **Update `cmd/skill.md`, `README.md` and the usage text in `cmd/help.go` in
  the same commit** as any change to the CLI surface. `skill.md` is embedded in
  the binary and printed by `trafaret skill`; a test fails when a command or flag
  is missing from it.
- **A changed mechanism means changed site prose.** The operator site's static
  text is in `internal/site/strings.go`; update the English and Russian maps
  together. `go test` enforces key parity.
- **Keep the `go` directive at 1.24.** A newer directive forces a toolchain
  download on every user below it; `go get` can raise it silently, so re-check
  `go.mod` after any dependency bump.
- **Conventional commit subjects** — `feat:`, `fix:`, `docs:`, `refactor:`,
  `test:`, `chore:`.

## Releases

Maintainer-only:

```bash
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

GoReleaser builds archives for linux/darwin/windows on amd64 and arm64. The
tag is stamped into the binary as `cmd.version`, which `trafaret version` prints
and the manifest `engine:` gate reads.
