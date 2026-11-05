---
type: Decision
title: Rename pastefix to pastewise
description: The second rename (2026-11-05), the name check, and why old settings move by hand instead of through migration code.
tags: [decision, naming]
generated:
  by: claude-code/claude-opus-5-5
  at: '2026-11-05T12:00:00Z'
---

# Decision

On 2026-11-05 the user renamed `pastefix` to `pastewise` and picked the Alfred keyword
`pw`. The tool now reads tables from many apps, converts between every format and
draws a picture, so it does more than fix a paste. This follows the first
[rename from tablefix](/decisions/rename-pastefix.md).

Checked the same day, by exact repo name on GitHub and Homebrew formula:

| Name | GitHub repos | Homebrew |
|---|---|---|
| `pastewise` | 2 | free |
| `pastekit` | 5 | free |
| `pasteup` | 14 | free |
| `paster` | 47 | free |

# What changed

- Go module `github.com/samhvw8/pastewise`, binary `pastewise`, env `PASTEWISE_CONFIG`
  and `PASTEWISE_CLAUDE`.
- Config folder `~/Library/Application Support/pastewise`, keychain service `pastewise`.
- Alfred bundle id `com.github.samhvw8.pastewise`. `alfred install` also matches
  `com.github.samhvw8.pastefix` and `com.github.samhvw8.tablefix`, so it updates the
  installed workflow in place. The default keyword is `pw`.
- History in `log.md` keeps the old names.

# No migration code

The repo had 0 stars and 0 forks, so the only real install was the user's. Code that
moves the old config folder and keychain item would run once and then stay as a
maintenance cost. Instead the folder (settings, usage log, last target) moves by hand
at upgrade time, and the README tells anyone else how. The user's keychain had no
`pastefix` API key, so nothing moves there.
