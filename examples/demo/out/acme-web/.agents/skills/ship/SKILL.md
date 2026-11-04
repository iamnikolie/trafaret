---
name: ship
description: Use when a change in acme-web is ready to land — runs the 2 gates, writes the commit with its jira reference, and opens the review against develop. Applies only to the acme-web repo.
---

# ship — land a change in acme-web

Acme · TypeScript · target branch `develop`.

## 1 · Gates

Every gate green before anything else:

- **test** — `pnpm test`
- **types** — `pnpm tsc --noEmit`
- **e2e** (slow, last) — `pnpm playwright test`

All of them in order, stopping at the first red: `.agents/skills/ship/scripts/gates.sh`.

Want a second pair of eyes? Run the `gatekeeper` custom agent (`.codex/agents/gatekeeper.toml`).

## 2 · Commit

One commit, message `<what changed> (<ref>)`, where the ref is the issue — e.g.
`WEB-123`, linked as https://acme.atlassian.net/browse/WEB-<n>.

## 3 · Review

Open the review against `develop`; it needs 2 approvals.
Your open reviews: `.agents/skills/ship/scripts/my-reviews.sh`.
