---
type: Architecture
title: Output targets and clipboard
description: The six targets, how input and output pick stdin, stdout or the clipboard, and how rich text and images move through the pasteboard.
tags: [output, clipboard, macos]
resource: clipboard.go
generated:
  by: claude-code/claude-opus-5-5
  at: '2026-11-05T18:30:00Z'
---

# Targets

| Target | Output |
|---|---|
| `notes` | HTML table + RTF on the clipboard, box table as the plain-text fallback |
| `slack` | Box table in a `````` code block, cells wrapped at `--width` (default 40) |
| `list` | One block per row: bold first cell, then `• Header: value` bullets (`--bold` sets the marker) |
| `box` | Box table without the fence |
| `md` | Aligned Markdown table (`Column N` names when there is no header) |
| `image` | PNG drawn by AppKit; on the clipboard as PNG + TIFF with no text; piped, the raw PNG bytes ([decision](/decisions/image-via-appkit.md)) |

# Input and output

- Input is stdin only when stdin is a FIFO or a regular file; otherwise the clipboard.
  Which table is read from it: [parsing](/architecture/parsing.md). Checking for a terminal instead hung forever under agent shells and
  Alfred, where stdin is an open non-terminal that never closes.
- Output goes to the clipboard when stdout is a terminal (`x/term`), or with `--copy`;
  otherwise it is printed. `--stdout` forces printing. `image` refuses to print PNG
  bytes to a terminal.
- The target may come first or after the flags (`pastefix --width 30 slack`). Clipboard
  use off macOS fails with a message that says to pipe instead.

# Reading the clipboard

One JXA script (`readJXA` in `clipboard.go`) writes the raw bytes of
`public.utf8-plain-text`, `public.html` and `public.rtf` to a temp folder and prints
which types it found, plus `image` when `public.png` or `public.tiff` is there. Bytes
in files, not text on stdout, keep non-ASCII safe without a locale. HTML with a
UTF-16 byte-order mark is decoded. When there is no HTML table but the RTF holds one
(`\trowd`), `textutil -convert html` turns it into HTML. A read takes about 80 ms, so
the Alfred filter can still run it on every keystroke.

# Rich-text clipboard

Every macOS tool runs by absolute path (`/usr/bin/…`) with `LANG`/`LC_ALL=en_US.UTF-8`.
Alfred gives scripts no PATH guarantee and no `LANG`, and without a locale the old
`pbpaste` reader turned every non-ASCII character into `?`, so a `│` table read as "no table". The
Python version escaped this only because Python sets `LC_CTYPE` for child processes
(PEP 538). `mise run test:clipboard` checks the round trip with an empty environment.


`textutil -convert rtf` turns the HTML table into RTF (a real `\trowd`/`\cell` table).
`clipboard.go` writes the RTF to a temp file, then a JXA script (`osascript -l JavaScript`) clears `NSPasteboard` and sets
`public.html`, `public.rtf` and `public.utf8-plain-text` in one go, so each app takes
the richest type it understands.

# Image clipboard

`writeImageJXA` (`image.go`) clears the pasteboard and sets `public.png` and
`public.tiff` only. With text on the clipboard too, apps such as Slack paste the text
instead of the picture. Checked on this Mac: `NSImage initWithPasteboard` (what an app
calls on paste) reads the image back at 445×154 pt from a 2x PNG.
