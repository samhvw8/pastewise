---
type: Reference
title: Getting started — pastewise
description: What pastewise is, how its parts fit, and where each piece of knowledge lives.
tags: [getting-started]
status: stable
generated:
  by: claude-code/claude-opus-5-5
  at: '2026-11-05T18:30:00Z'
---

# Overview

pastewise turns a table (Markdown pipes, box-drawing lines, tab-separated cells, its own
list, or a rich-text table copied from Notes, Docs, Sheets or a web page) into a form
that pastes well in the target app: a rich-text table for Apple Notes and other editors,
a code-block box table for Slack, a list for chat boxes, or a PNG picture. Every text
target reads back in, so any output converts to any other. An optional AI step (`--ai`)
then lets a model improve the text, while code still draws the table. It is one Go binary (a
`main` package at the repo root) and installs itself into Alfred as a workflow.
It began as a Python script named tablefix; see [the Go rewrite](/decisions/go-rewrite.md)
and the renames to [pastefix](/decisions/rename-pastefix.md) and
[pastewise](/decisions/rename-pastewise.md).

It was built after an AI-made box table pasted into Apple Notes showed up as rows of
pipes with drawn lines between them (Notes renders `├───┼───┤` as strokes).

# Layout

| Path | What it is |
|---|---|
| `table.go` | `Parse` (pipe and box tables) and the renderers |
| `input.go` | `readTable` and the HTML, tab-separated and list parsers |
| `clipboard.go` | macOS clipboard read (text, HTML, RTF) and rich-text write |
| `image.go`, `image.js` | The `image` target: AppKit draws the PNG |
| `ai.go` | The optional `--ai` step: Anthropic API with a key, else `claude -p`; checks the returned shape |
| `config.go` | Settings file, API key in the keychain, `config` and `prompt` commands |
| `alfred.go` | `pastewise alfred …`: writes, finds and removes the workflow |
| `usage.go` | Temporary usage log, `pastewise stats`, the last-target default |
| `main.go` | CLI flags and the stdin/stdout/clipboard rules |
| `*_test.go` | Go tests (fake runner, fake keychain, local API server) |

# Concepts

- [Parsing](/architecture/parsing.md): which inputs are read (rich text, list, tab-separated, pipe, box), how rows and the header are found.
- [Output and clipboard](/architecture/output.md): the six targets and how the clipboard is read and written.
- [AI step](/architecture/ai-step.md): `--ai clean` or a free instruction; the model edits data, code draws.
- [Alfred workflow](/architecture/alfred-workflow.md): how the CLI writes the `pw` workflow; ⌥-Enter adds the AI step.
- [Settings and prompt shortcuts](/architecture/config.md): config file, API key in the keychain, `/` commands in Alfred.
- [One target per destination app](/decisions/targets-per-app.md): why Slack gets a code block or a list.
- [Go rewrite](/decisions/go-rewrite.md): why Go, and why the CLI installs the workflow.
- [Image via AppKit](/decisions/image-via-appkit.md): why the picture is drawn by AppKit, and why it is one way.
- [Rename to pastefix](/decisions/rename-pastefix.md): why, and how the old Alfred workflow is updated in place.
- [Rename to pastewise](/decisions/rename-pastewise.md): the second rename; what moves by hand.
