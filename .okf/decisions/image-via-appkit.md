---
type: Decision
title: Draw the image target with AppKit through JXA
description: Why the image target renders with AppKit in an osascript JXA script instead of a Go image library, and why the clipboard gets no text with it.
tags: [decision, image, macos]
resource: image.js
generated:
  by: claude-code/claude-opus-5-5
  at: '2026-11-05T18:30:00Z'
---

# Decision

`image.js` (embedded in the binary) draws the table with `NSString` drawing into an
`NSBitmapImageRep` at 2x, marks it 144 dpi, and writes a PNG. Go passes the table as
JSON in a temp file and reads the PNG back.

- **Fonts:** cells hold Vietnamese, CJK and emoji. AppKit picks a fallback font per
  character, as every Mac app does. A Go renderer (`x/image/font`) would need us to
  bundle fonts and write our own fallback.
- **No new dependency or build step:** `osascript` and AppKit ship with macOS, the same
  route the `notes` target already uses for the pasteboard.
- **Cost:** about 0.3 s per image, and the target works only on macOS. Off macOS it
  fails with a message; text targets still work there.

Layout: 14 pt system font (bold header on a light grey band), 12×8 pt cell padding,
1 pt grey grid, white background. Cells wrap at `--width` × 8 points.

# Clipboard: picture only

The clipboard gets `public.png` and `public.tiff` and no plain text. With text there,
Slack and other apps paste the text, not the picture. So `image` is one way:
pastewise cannot read a table from a picture yet, and an image on the clipboard as
input exits 1 with that message.

# Verified (2026-11-05, macOS 27.0.1)

A real render shows Vietnamese, CJK, an emoji and a wrapped cell correctly, and
`NSImage initWithPasteboard` reads the copied image back at its 1x size. A paste into
a running app was not checked: the apps were on a Space the session could not see.
