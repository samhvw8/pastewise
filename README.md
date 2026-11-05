# pastewise

*Formerly `pastefix`, and before that `tablefix`.*

Copy a table from ChatGPT, Claude, a terminal, Apple Notes, Google Docs or Sheets,
Excel or a web page, and paste it as a table that looks right where it lands.

Pasting a Markdown table (`| a | b |`) or a box-drawing table (`│ a │ b │`) into
Apple Notes, Slack or a chat box gives you a mess of pipes and lines. `pastewise`
reads the table from the clipboard and puts a better version back:

| Target | Where to paste | What you get |
|---|---|---|
| `notes` | Apple Notes, Pages, Mail, Google Docs, Notion, Word | A real table (rich text on the clipboard) |
| `slack` | Slack | An aligned box table in a code block |
| `list` | Messenger, Zalo, iMessage, WhatsApp, Slack | One block per row, with bullets |
| `md` | GitHub, Obsidian, anything Markdown | A clean, aligned Markdown table |
| `box` | Terminals, plain-text email | A box table without the code fence |
| `image` | Any app that takes a pasted picture | A PNG of the table (one way: pastewise cannot read it back yet) |

**Convert between them:** every target except `image` reads back in. Copy a Notes table
and run `pastewise slack`, or copy a `list` and run `pastewise md`.

Slack now turns cells pasted from Excel or Google Sheets (tab-separated text) into a
real table, and one source says a pasted Markdown table works too. `slack` stays as the
code block, because it shows the same everywhere. See [Host tests](#host-tests) for what
was tested on a real Mac.

## Install

A single Go binary with no runtime dependencies:

```bash
go install github.com/samhvw8/pastewise@latest
# or, with mise
mise use -g go:github.com/samhvw8/pastewise
```

Clipboard use and the `image` target are macOS only (`osascript`, `textutil`). On
Linux, pipe text in and out: `pastewise md < table.txt`.

Upgrading from `pastefix`: install `pastewise`, move
`~/Library/Application Support/pastefix` to `~/Library/Application Support/pastewise`,
run `pastewise alfred install` (it updates the old workflow; the keyword becomes `pw`),
then remove the old binary. An API key saved with `pastefix config set apikey` must be
set again with `pastewise config set apikey`.

### Alfred workflow

The CLI writes the workflow into Alfred for you, pointing at its own path (Alfred
runs scripts without your shell `PATH`). It needs the Alfred Powerpack.

```bash
pastewise alfred install            # or --keyword tbl, --bin /path/to/pastewise
pastewise alfred uninstall
pastewise alfred export -o pastewise.alfredworkflow   # a file to import by hand
```

Running `install` again updates the same workflow in place. Then:

1. Copy a table.
2. Type `pw` in Alfred. Each target shows how many rows it found on the clipboard.
3. Choose a target and press Enter.
4. Paste with ⌘V.

Press ⌥-Enter instead of Enter to add the [AI step](#ai-step-optional).

To rewrite the table with your own instruction, type it after `pw`:

| You type | Result |
|---|---|
| `pw slack translate to Vietnamese` | Slack only; Enter runs the instruction |
| `pw shorten each cell` | Every target; Enter runs the instruction |
| `pw slack vi` | Runs your saved shortcut `vi` (see [Settings](#settings-and-shortcuts)) |
| `pw no` | `notes`, with no AI step |
| `pw /` | Settings commands: `/config`, `/set apikey KEY`, `/prompt add NAME TEXT`, … |

If you move the binary, run `pastewise alfred install` again.

## Use

```bash
pastewise notes                                 # clipboard -> clipboard, then ⌘V in Notes
pastewise slack --width 30                      # wrap long cells at 30 characters
pastewise list --header "Field,Journals,Magazines"
pastewise list --bold "**"                      # Markdown bold instead of Slack *bold*
pbpaste | pastewise md > table.md               # piped: prints instead of copying
pastewise image                                 # a PNG of the table, then ⌘V anywhere
pastewise image < table.md > table.png          # piped: writes the PNG bytes
pastewise                                       # no target: the last one you copied, else notes
pastewise stats                                 # runs per target, from the usage log
```

- **Input:** stdin when it is a pipe or a file, else the clipboard. pastewise reads, in
  this order:
  - a rich-text table: HTML (Google Docs, Sheets, Excel, web pages, `pastewise notes`) or
    RTF (Apple Notes). It keeps the header: a `<thead>`, a row of `<th>`, or a bold
    first row. Merged cells leave empty cells, so the columns stay in line.
  - a `list` output: a bold title line, then `• Key: value` bullets. The keys become the
    header; the title column's name is empty, because the list never shows it.
  - tab-separated cells (Excel, Sheets, Numbers, the plain text of a Notes copy). Plain
    text says nothing about a header, so there is none; add one with `--header`.
  - a Markdown or box table.
  - An image on the clipboard exits with an error: reading a table from a picture is not
    supported yet.
- **Image:** cells wrap at `--width` × 8 points, drawn at 2x with the macOS system font,
  so Vietnamese, CJK and emoji all show. Only the PNG goes on the clipboard, with no
  text, so apps such as Slack paste the picture and not the text.
- **Output:** the clipboard when run in a terminal (with a preview), else stdout.
  `--copy` forces the clipboard; `--stdout` forces printing.
- **Header:** detected when only the first row has a rule under it, or a heavy one
  (`═`, `━`, `+===+`). Rules after every row mean there is no header. Override with
  `--header "A,B,C"` or `--no-header`.
- **Wrapped cells:** in a box table, the lines between two rules are one row, so
  cells wrapped over several lines are joined again. `--width` wraps at spaces only,
  so a re-read gives back the same text.
- No table found, or a `--header` with the wrong number of names, exits with an
  error and changes nothing.

## AI step (optional)

After the table is fixed in code, a model can improve its text. The code still draws
the table, so the model cannot break it. It runs one of two ways:

- **No API key (default):** through the [Claude Code](https://claude.com/claude-code)
  CLI on your Claude login. No extra cost; about 5 seconds.
- **With an API key** (`pastewise config set apikey KEY`): directly on the Anthropic
  API. Billed to the key; Haiku costs well under a cent a run.

```bash
pastewise notes --ai clean                        # join wrapped words, fix typos, add a missing header
pastewise list --ai "translate to Vietnamese"     # any other text is the instruction
pastewise list --ai vi                            # a saved shortcut
pastewise md --ai clean --ai-model sonnet         # default model: haiku, or the config model
```

`clean` keeps every row, column and fact; pastewise rejects an answer with a different
shape. In Alfred, press ⌥-Enter instead of Enter, or type an instruction after `pw`. One run takes about 5 seconds. If
the AI step fails (no `claude`, not logged in, a bad answer), pastewise says why and
copies the table fixed in code.

```
| Column 1 | Column 2                                 |     →     | Name  | Venue                                        |
| TOCHI    | ACM Computer- Human Interac tion journal |  --ai     | TOCHI | ACM Computer-Human Interaction journal       |
| PSPI     | Psychological Science in th e Public ... |  clean    | PSPI  | Psychological Science in the Public Interest |
```

## Settings and shortcuts

```bash
pastewise config                                  # list every setting
pastewise config set apikey sk-ant-...            # stored in the macOS keychain, never in a file
pastewise config unset apikey                     # back to your Claude login
pastewise config set model sonnet                 # haiku, sonnet, opus or a model id
pastewise prompt add vi translate to Vietnamese   # shortcut: --ai vi, or pw slack vi
pastewise prompt rm vi
```

In Alfred the same commands start with `/`: `pw /config`, `pw /set apikey sk-ant-...`,
`pw /prompt add vi translate to Vietnamese`. With an empty query, Alfred lists your
shortcuts under the targets. Settings live in
`~/Library/Application Support/pastewise/config.json`.

## Usage log

Until 2026-11-28, each run adds one line to `usage.jsonl` next to the settings file:
target, input kind (pipe, box, wrapped, html, tsv, list, image), exit code, whether the AI step fell back, and
whether it came from Alfred. Nothing leaves the Mac. `pastewise stats` marks a target that
nobody used as "candidate to cut", and one that only failed as "fix or cut". After the end
date the log stops writing, and `stats` asks for the cut decision.

## Host tests

A target stays only while the app it serves still shows a raw pasted table wrong. Test
input: `| Name | Score |`, `|---|---|`, `| a | 1 |`, and the same table as tab-separated
text.

| Target | App | Raw pipe table | Tab-separated | With pastewise | Tested |
|---|---|---|---|---|---|
| `notes` | Apple Notes, macOS 27.0.1 | plain text | plain text | real table | 2026-10-29 |
| `slack` | Slack desktop | not tested | not tested | code block | Slack was signed out |

## Examples

`pastewise slack --width 24`:

```
┌──────────┬────────────────────────┬─────────────────────────┐
│ Field    │ Journals               │ Magazines               │
╞══════════╪════════════════════════╪═════════════════════════╡
│ Learning │ PSPI, Educational      │ Behavioral Scientist    │
│          │ Psychology Review      │                         │
├──────────┼────────────────────────┼─────────────────────────┤
│ Habits   │ Nature Human Behaviour │ Harvard Business Review │
└──────────┴────────────────────────┴─────────────────────────┘
```

`pastewise list`:

```
*Learning*
• Journals: PSPI, Educational Psychology Review
• Magazines: Behavioral Scientist

*Habits*
• Journals: Nature Human Behaviour
• Magazines: Harvard Business Review
```

## Develop

```bash
mise run test      # go test -race
mise run check     # gofmt, vet (macOS + Linux), tests, docs validation
mise run build     # bin/pastewise
mise run release   # tag and publish macOS and Linux binaries
```

## License

MIT
