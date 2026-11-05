---
type: Architecture
title: Parsing text tables
description: How pastewise finds a table in rich text (HTML, RTF), a list, tab-separated text, or a Markdown or box table, and detects the header.
tags: [parser]
resource: input.go
generated:
  by: claude-code/claude-opus-5-5
  at: '2026-11-05T18:30:00Z'
---

# Which parser reads the input

`readTable` (`input.go`) tries the parsers in this order and logs which one won:

| Order | Input | Kind in the log | Header |
|---|---|---|---|
| 1 | HTML with a `<table>`: the clipboard's `public.html`, an RTF table converted by `textutil`, or stdin text that starts with `<` | `html` | `<thead>`, a row of all `<th>`, or a first row that is all bold (`<b>`, `<strong>`, `font-weight` 600–900 or bold) while a later row is not |
| 2 | The `list` target's output: blocks split by blank lines, a title line, then only `• `/`- `/`* ` bullets | `list` | When every bullet is `Key: value`, the keys in first-seen order; the title column's name is empty |
| 3 | Tab-separated text, when more lines have tabs than pipes | `tsv` | None: plain text does not say |
| 4 | A Markdown or box table (rules below) | `pipe`, `box`, `wrapped` | See below |
| 5 | Tab-separated text, as a last try | `tsv` | None |

Rich text goes first because it keeps the header that the tab-separated plain text of
the same copy loses. `Parse` takes any line with a `|`, so the stricter list and
tab shapes go before it: a `|` inside a list value or a spreadsheet cell would
otherwise pass for a pipe table. If nothing parses and the clipboard holds an image,
the error says that reading a table from a picture is not supported yet (kind `image`).

**HTML details:** only the first top-level `<table>` is read; a nested table's text
belongs to its cell. A cell's text goes on one line (`<br>` and paragraph breaks
become spaces). `colspan` and `rowspan` leave empty cells so the columns stay in
line. Rows with only empty cells are dropped.

**TSV details:** lines without a tab are skipped: a Notes copy starts with the note's
title, and spreadsheets keep the tabs of empty cells.

**Apple Notes** (tested on macOS 27.0.1, ⌘A ⌘C in a note with a table) puts
`com.apple.notes.richtext`, RTFD, `public.rtf` and tab-separated plain text on the
clipboard, and no HTML. `textutil -convert html` keeps the RTF table as `<table>` with
bold header cells. The real RTF is `testdata/notes-copy.rtf`.

Every text target reads back to the same rows, so any output converts to any other
(`TestEveryTargetReadsBack`). Two losses are known: `list` drops the first column's
name, and `md` names the columns `Column N` when there is no header.

# Rules for Markdown and box tables

- `│`, `┃` and `║` are read as `|`. A line with `|` and some text is a data line.
  `\|` stays a literal pipe inside a cell. In a table drawn with `│`, an ASCII `|` is
  cell text, so a box table with `a | b` in a cell reads back the same.
- A **rule line** has only rule characters (`─━═=-+:|` and the box corners and joints).
  It is a box rule if it has a box character, `=` or `+`; a Markdown separator if it
  has only `-`, `:` and `|`.
- **Box mode** (any box rule present): the data lines between two rules form one row,
  and their cell fragments are joined with a space. This rejoins cells that a terminal
  or pastewise's own `--width` wrapped. In Markdown mode each data line is a row.
- Short rows are padded with empty cells to the widest row.
- **Header:** the first row is a header when the only rule between rows is after row 1,
  or when the rule after row 1 is heavy (`═`, `━`, `=`). Rules after every row (the
  Apple Notes case) mean no header. A two-row table with one rule is read as having a
  header; `--no-header` overrides, and `--header "A,B"` names the columns.
- No row found, or a `--header` count that differs from the column count, exits 1
  with a message and writes nothing.

The box renderer draws the header rule as `╞═╪═╡`, so its own output parses back to
the same header and rows (tested).

Column widths use `go-runewidth`: East Asian wide characters count 2, combining marks 0.
`--width` wraps at spaces only; a hyphenated word stays whole, so a re-read joins the
lines back to the same text (the Python version split at hyphens and lost that).
