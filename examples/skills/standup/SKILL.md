---
name: standup
description: Use when asked for a standup or "what did I do" — summarises the last working day from git history across the repos you name. Plain Markdown with no template, so it is copied as-is into every harness.
---

# standup

1. For each repo the user names: `git log --since=yesterday --author="$(git config user.email)" --oneline`.
2. Group by repo, one line per change, newest first.
3. End with what is blocked, if anything was said about it.
