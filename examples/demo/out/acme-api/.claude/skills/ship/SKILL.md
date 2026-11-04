---
name: ship
description: Use when a change in acme-api is ready to land — runs the 2 gates, writes the commit with its github reference, and opens the review against main. Applies only to the acme-api repo.
---

# ship — land a change in acme-api

Acme · Go · target branch `main`.

## 1 · Gates

Every gate green before anything else:

- **test** — `go test ./...`
- **lint** — `golangci-lint run`

All of them in order, stopping at the first red: `.claude/skills/ship/scripts/gates.sh`.

Want a second pair of eyes? Dispatch the `gatekeeper` subagent — it re-runs the gates read-only.

## 2 · Commit

One commit, message `<what changed> (<ref>)`, where the ref is the issue — e.g.
`#123`, linked as https://github.com/acme/api/issues/<n>.

## 3 · Review

Open the review against `main`; it needs 1 approval.
Your open reviews: `.claude/skills/ship/scripts/my-reviews.sh`.
