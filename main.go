// Command pastewise turns a table (Markdown, box-drawing, tab-separated, a list, or
// a rich-text table copied from Notes, Docs, Sheets or a web page) into one that
// pastes well elsewhere: a rich-text table for Apple Notes, a code-block table for
// Slack, a list for chat boxes, or a picture.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

const version = "0.7.0"

const usage = `pastewise: turn a table into one that pastes well elsewhere.

Usage:
  pastewise [target] [flags]
  pastewise stats                           use per target, from the usage log
  pastewise config                          list every setting
  pastewise config set apikey|model VALUE   apikey goes to the macOS keychain
  pastewise config unset apikey|model
  pastewise prompt add NAME TEXT...         save a shortcut: --ai NAME runs TEXT
  pastewise prompt rm NAME
  pastewise alfred install [--bin PATH] [--keyword pw]
  pastewise alfred uninstall | export [-o FILE] | filter | run

Targets:
  notes  rich-text table (HTML + RTF) -> native table in Apple Notes, Pages,
         Mail, Google Docs, Notion, Word
  slack  box table inside a code block (Slack messages have no tables)
  list   one block per row, for chat boxes (Messenger, Zalo, iMessage, Slack)
  box    box table without the code fence
  md     clean, aligned Markdown table
  image  PNG picture of the table, for any app that takes a pasted image;
         piped, it writes the PNG bytes (pastewise image < t.md > t.png)
With no target, pastewise reuses the last target you copied to the clipboard,
else notes. Piped output (a script) always defaults to notes.

Input: a Markdown or box table, tab-separated cells (Excel, Sheets, Numbers),
the list target's output, or a rich-text table copied from Apple Notes, Google
Docs, Sheets, Excel or a web page. Every target except image reads back in, so
any output converts to any other. A picture cannot be read back yet.

AI step (optional): after the table is fixed in code, a model edits its text.
The code still draws the table, so the model cannot break it.
  --ai clean              join words split by wrapping, fix typos, add a missing
                          header; keeps every row, column and fact
  --ai NAME               a shortcut saved with "prompt add"
  --ai "<instruction>"    anything else, e.g. --ai "translate to Vietnamese"
With an API key set, it calls the Anthropic API; else it runs the claude CLI on
your Claude login. If the AI step fails, pastewise says why and uses the table
fixed in code.

Input is stdin when it is a pipe or file, else the clipboard. Output goes to the
clipboard when run in a terminal (preview on stderr), else to stdout.

Flags:
`

// Swapped in tests.
var (
	clipboardReader = readClipboard
	clipboardWriter = writeClipboard
	imageRenderer   = renderImage
	imageWriter     = writeImageClipboard
	stdinPiped      = func() bool {
		fi, err := os.Stdin.Stat()
		return err == nil && (fi.Mode()&os.ModeNamedPipe != 0 || fi.Mode().IsRegular())
	}
	stdoutTerminal = func() bool { return term.IsTerminal(int(os.Stdout.Fd())) }
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(os.Stderr, "pastewise:", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "alfred" {
		return runAlfred(args[1:], stdout, stderr)
	}
	if len(args) > 0 && (args[0] == "config" || args[0] == "prompt") {
		msg, err := loggedConfigCommand(args, stdout, stderr, "cli")
		if msg != "" {
			fmt.Fprintln(stderr, "pastewise: "+msg)
		}
		return err
	}
	if len(args) == 1 && args[0] == "stats" {
		return statsCommand(stdout)
	}
	return convertLogged(args, stdin, stdout, stderr, "cli")
}

// loggedConfigCommand runs a config or prompt command and adds it to the usage log.
func loggedConfigCommand(words []string, out, warn io.Writer, via string) (string, error) {
	msg, err := configCommand(words, out)
	r := usageRow{Target: words[0], Via: via}
	if err != nil {
		r.Exit = 1
	}
	logUsage(r, warn)
	return msg, err
}

// convertLogged runs one conversion and adds it to the usage log.
func convertLogged(args []string, stdin io.Reader, stdout, stderr io.Writer, via string) error {
	r := usageRow{Via: via}
	err := convert(args, stdin, stdout, stderr, &r)
	if r.Target != "" { // --help and --version are not conversions
		if err != nil {
			r.Exit = 1
		}
		logUsage(r, stderr)
	}
	return err
}

func convert(args []string, stdin io.Reader, stdout, stderr io.Writer, r *usageRow) error {
	target, positional := "", false
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		target, args, positional = args[0], args[1:], true
	}
	fs := flag.NewFlagSet("pastewise", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage); fs.PrintDefaults() }
	header := fs.String("header", "", "comma-separated column names (overrides detection)")
	noHeader := fs.Bool("no-header", false, "treat the first row as data")
	wrapAt := fs.Int("width", 40, "wrap box cells at N columns (0 = no wrap)")
	bold := fs.String("bold", "*", `bold marker for list titles: "*" Slack, "**" Markdown, "" none`)
	toStdout := fs.Bool("stdout", false, "print instead of copying to the clipboard")
	toClip := fs.Bool("copy", false, "copy to the clipboard even when stdout is not a terminal")
	showVersion := fs.Bool("version", false, "print the version")
	aiStep := fs.String("ai", "", `AI step after the code fix: "clean" or a free instruction`)
	aiModel := fs.String("ai-model", "", "model for --ai (default: the config model, else haiku)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, "pastewise", version)
		return nil
	}
	if fs.NArg() > 0 { // target after the flags: pastewise --width 30 slack
		if positional || fs.NArg() > 1 {
			return fmt.Errorf("unexpected argument %q", fs.Arg(fs.NArg()-1))
		}
		target = fs.Arg(0)
	}
	toClipboard := !*toStdout && (*toClip || stdoutTerminal())
	explicit := target != ""
	if !explicit {
		// No target: reuse the last one copied to the clipboard. A piped run is a
		// script, so it always gets notes.
		target = "notes"
		if toClipboard {
			if last, err := lastTarget(); err != nil {
				fmt.Fprintf(stderr, "pastewise: last target: %v; using notes\n", err)
			} else if last != "" {
				target = last
			}
		}
	}
	switch target {
	case "notes", "slack", "list", "box", "md", "image":
	default:
		return fmt.Errorf("unknown target %q (notes, slack, list, box, md, image)", target)
	}
	if target == "image" && !toClipboard && stdoutTerminal() {
		return errors.New("image writes PNG bytes; redirect them to a file (pastewise image --stdout > table.png)")
	}
	r.Target, r.Input = target, "none"

	var in source
	if stdinPiped() {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		in.Text = string(b)
	} else {
		var err error
		if in, err = clipboardReader(); err != nil {
			return err
		}
	}

	t, kind, err := readTable(in)
	r.Input = kind
	if err != nil {
		return err
	}
	aiNote := ""
	if *aiStep != "" {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		model := *aiModel
		if model == "" {
			model = cfg.Model
		}
		if model == "" {
			model = defaultModel
		}
		// A failed AI step falls back to the code fix, and says so.
		r.AI = aiLabel(*aiStep, cfg.Prompts)
		if et, err := Enhance(t, *aiStep, model, cfg.Prompts); err != nil {
			r.AIFallback = true
			aiNote = fmt.Sprintf(" AI step failed (%v); used the code fix.", err)
			fmt.Fprintln(stderr, "pastewise:"+aiNote)
		} else {
			t, aiNote = et, " AI step: "+aiLabel(*aiStep, cfg.Prompts)+"."
		}
	}
	if *noHeader {
		t.SetHeader(nil) //nolint:errcheck // nil never fails
	}
	if *header != "" {
		names := strings.Split(*header, ",")
		for i := range names {
			names[i] = strings.TrimSpace(names[i])
		}
		if err := t.SetHeader(names); err != nil {
			return err
		}
	}

	var out, rich string
	var png []byte
	switch target {
	case "notes":
		out, rich = Box(t, *wrapAt), HTML(t)
	case "slack":
		out = "```\n" + Box(t, *wrapAt) + "\n```"
	case "box":
		out = Box(t, *wrapAt)
	case "list":
		out = List(t, *bold)
	case "md":
		out = Markdown(t)
	case "image":
		out = Box(t, *wrapAt) // the preview on stderr
		if png, err = imageRenderer(t, *wrapAt); err != nil {
			return err
		}
	}

	if !toClipboard {
		if png != nil {
			_, err := stdout.Write(png)
			return err
		}
		_, err := fmt.Fprintln(stdout, out)
		return err
	}
	if png != nil {
		err = imageWriter(png)
	} else {
		err = clipboardWriter(out, rich)
	}
	if err != nil {
		return err
	}
	if explicit {
		if err := saveLastTarget(target); err != nil {
			fmt.Fprintf(stderr, "pastewise: last target: %v\n", err)
		}
	}
	fmt.Fprintln(stderr, out)
	copied := target
	switch {
	case rich != "":
		copied = "rich-text table"
	case png != nil:
		copied = "an image"
	}
	fmt.Fprintf(stderr, "\npastewise: copied %d rows as %s.%s Paste with ⌘V.\n", len(t.Rows), copied, aiNote)
	return nil
}

// aiLabel names the step in the copy message: the built-in step or shortcut
// name, else "custom".
func aiLabel(step string, prompts map[string]string) string {
	if _, ok := aiSteps[step]; ok {
		return step
	}
	if _, ok := prompts[step]; ok {
		return step
	}
	return "custom"
}
