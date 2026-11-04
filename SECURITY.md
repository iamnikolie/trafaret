# Security Policy

## Supported versions

The latest release is the supported one. Fixes land on `main` and go out in the
next tag.

## Reporting a vulnerability

Please do **not** open a public issue for a security problem.

Use GitHub's private vulnerability reporting:
[Security → Report a vulnerability](https://github.com/iamnikolie/trafaret/security/advisories/new).

trafaret writes files into your repositories and your harness directories, so
**a write outside the paths it is meant to manage** is the vulnerability this
project cares about most. If you find a way to make `install` create, overwrite
or delete a file outside the project's repo root or a home's skills directory —
a crafted manifest, template, track file or `bundles.yaml` — that is a report
worth making, and a minimal config that shows it is enough.

Expect a first response within a week. This is a spare-time project, not a
product with an on-call rotation.

## Scope notes

Known and by design rather than vulnerabilities:

- **Templates and the manifest are trusted input.** They are Go `text/template`
  files and YAML you wrote, read from your own config directory. Rendering a
  library you do not trust writes whatever that library says into your repos;
  read it first, and use `trafaret diff` before `trafaret install`.
- **Prune deletes, but only what trafaret recorded.** It removes paths listed in
  its own track files, and only under `.claude/`, `.cursor/`, `.agents/` and
  `.codex/` in a repo (or inside a home's skills directory). Paths containing
  `..` are ignored. A hand-edited track file is as trusted as the rest of your
  config directory.
- **The session hook runs a shell command.** A bundle's `sessionContext` becomes
  a `cat` command in the project's `.claude/settings.json`. The path is
  restricted to a plain relative path under `.claude/skills/` — no `..`, no
  quotes, `$` or backticks — but the file it prints is whatever the bundle
  ships.
- **`serve` is a local development aid.** It binds the `--addr` you give
  (default `127.0.0.1:4477`), has no authentication, and renders your manifest
  and template catalog on every request. Do not bind it to a public interface.
- **Rendered output may contain what your manifest contains.** Hostnames,
  workspace names and ticket-board names land in committed files. Keep secrets
  out of the manifest.
