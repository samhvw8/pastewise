package main

import (
	"bytes"
	"errors"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var sample = Table{
	Header: []string{"Name", "Role", "Note"},
	Rows: [][]string{
		{"Alice", "Dev", "Tiếng Việt có dấu, 中文"},
		{"Bob", "", "a: b | c"},
		{"Carol", "PM", "a long note that the box target wraps over several lines"},
	},
}

func sameRows(a, b [][]string) bool {
	return slices.EqualFunc(a, b, func(x, y []string) bool { return slices.Equal(x, y) })
}

// Every text target reads back to the same table, so any output converts to
// any other. The list never shows the first column's name, so it reads back empty.
func TestEveryTargetReadsBack(t *testing.T) {
	tsv := "Name\tRole\tNote\nAlice\tDev\tTiếng Việt có dấu, 中文\nBob\t\ta: b | c\nCarol\tPM\ta long note that the box target wraps over several lines\n"
	cases := []struct {
		name   string
		in     source
		kind   string
		header []string
	}{
		{"slack", source{Text: "```\n" + Box(sample, 12) + "\n```"}, "wrapped", sample.Header},
		{"box", source{Text: Box(sample, 0)}, "box", sample.Header},
		{"md", source{Text: Markdown(sample)}, "pipe", sample.Header},
		{"notes (rich)", source{Text: Box(sample, 40), HTML: HTML(sample)}, "html", sample.Header},
		{"notes (html file on stdin)", source{Text: HTML(sample)}, "html", sample.Header},
		{"list slack", source{Text: List(sample, "*")}, "list", []string{"", "Role", "Note"}},
		{"list markdown", source{Text: List(sample, "**")}, "list", []string{"", "Role", "Note"}},
		{"list plain", source{Text: List(sample, "")}, "list", []string{"", "Role", "Note"}},
		{"tsv", source{Text: "Title of the note\n\n" + tsv}, "tsv", nil},
	}
	for _, c := range cases {
		got, kind, err := readTable(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		want := sample.Rows
		if c.header == nil {
			want = append([][]string{sample.Header}, sample.Rows...)
		}
		if kind != c.kind || !slices.Equal(got.Header, c.header) || !sameRows(got.Rows, want) {
			t.Fatalf("%s: kind %s header %q rows %q", c.name, kind, got.Header, got.Rows)
		}
	}
}

func TestParseListWithoutKeys(t *testing.T) {
	got, err := ParseList(List(Table{Rows: [][]string{{"a", "b", "c"}, {"d", "e", ""}}}, "*"))
	if err != nil || got.Header != nil || !sameRows(got.Rows, [][]string{{"a", "b", "c"}, {"d", "e", ""}}) {
		t.Fatalf("got %q err %v", got.Rows, err)
	}
	for _, text := range []string{"hello", "one line\nand prose under it", "- a\n- b"} {
		if _, err := ParseList(text); err == nil {
			t.Fatalf("%q read as a list", text)
		}
	}
}

// What Google Sheets, Excel and web pages put on the clipboard: bold set by
// style, merged cells, a nested table, line breaks inside a cell.
func TestParseHTMLSpreadsheetShapes(t *testing.T) {
	const sheet = `<meta charset="utf-8"><google-sheets-html-origin><style>td{}</style>
<table><tbody>
<tr><td style="font-weight:bold">Team</td><td style="font-weight:700"><span>Who</span></td><td><b>When</b></td></tr>
<tr><td rowspan="2">Core</td><td>Ann<br>Lee</td><td>Mon</td></tr>
<tr><td colspan="2">Ben (all week)</td></tr>
<tr><td>Ops</td><td><table><tr><td>inner</td></tr></table></td><td><p>Tue</p><p>Wed</p></td></tr>
</tbody></table>`
	got, err := ParseHTML(sheet)
	want := [][]string{{"Core", "Ann Lee", "Mon"}, {"", "Ben (all week)", ""}, {"Ops", "inner", "Tue Wed"}}
	if err != nil || !slices.Equal(got.Header, []string{"Team", "Who", "When"}) || !sameRows(got.Rows, want) {
		t.Fatalf("header %q rows %q err %v", got.Header, got.Rows, err)
	}

	// No bold first row, no th: no header.
	got, _ = ParseHTML(`<table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>`)
	if got.Header != nil || len(got.Rows) != 2 {
		t.Fatalf("header %q rows %q", got.Header, got.Rows)
	}
	// Every row bold: bold says nothing about a header.
	got, _ = ParseHTML(`<table><tr><td><b>a</b></td></tr><tr><td><b>c</b></td></tr></table>`)
	if got.Header != nil {
		t.Fatalf("header %q", got.Header)
	}
	// thead wins.
	got, _ = ParseHTML(`<table><thead><tr><td>h</td></tr></thead><tbody><tr><td>v</td></tr></tbody></table>`)
	if !slices.Equal(got.Header, []string{"h"}) {
		t.Fatalf("header %q", got.Header)
	}
	if _, err := ParseHTML(`<p>no table</p>`); !errors.Is(err, errNoTable) {
		t.Fatalf("err = %v", err)
	}
}

// testdata/notes-copy.rtf is the RTF Apple Notes (macOS 27.0.1) put on the
// clipboard after ⌘A ⌘C in a note holding a pasted table. Notes puts no HTML,
// so the reader converts the RTF with textutil.
func TestNotesCopyReadsWithHeader(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("textutil needs macOS")
	}
	h, err := exec.Command("/usr/bin/textutil", "-convert", "html", "-format", "rtf", "-stdout", "testdata/notes-copy.rtf").Output()
	if err != nil {
		t.Fatal(err)
	}
	tsv := "pastewise probe\npastewise probe\n\nName\tRole\tNote\nAlice\tDev\tTiếng Việt ok\nBob\tPM\ta, b\n"
	got, kind, err := readTable(source{Text: tsv, HTML: string(h)})
	want := [][]string{{"Alice", "Dev", "Tiếng Việt ok"}, {"Bob", "PM", "a, b"}}
	if err != nil || kind != "html" || !slices.Equal(got.Header, []string{"Name", "Role", "Note"}) || !sameRows(got.Rows, want) {
		t.Fatalf("kind %s header %q rows %q err %v", kind, got.Header, got.Rows, err)
	}
}

func TestDecodeHTMLUTF16(t *testing.T) {
	le := []byte{0xFF, 0xFE, '<', 0, 't', 0, 0xBF, 0x1E} // "<tế" little-endian
	if got := decodeHTML(le); got != "<tế" {
		t.Fatalf("got %q", got)
	}
	if got := decodeHTML([]byte("\xEF\xBB\xBF<p>")); got != "<p>" {
		t.Fatalf("got %q", got)
	}
}

func TestImageOnClipboardFailsLoudly(t *testing.T) {
	clipboardReader = func() (source, error) { return source{Image: true}, nil }
	wrote := false
	clipboardWriter = func(string, string) error { wrote = true; return nil }
	imageWriter = func([]byte) error { wrote = true; return nil }
	t.Cleanup(func() {
		clipboardReader, clipboardWriter, imageWriter = readClipboard, writeClipboard, writeImageClipboard
	})
	stdinPiped = func() bool { return false }
	stdoutTerminal = func() bool { return true }
	t.Cleanup(func() { stdinPiped = func() bool { return true } })
	var out, errOut bytes.Buffer
	if err := run([]string{"md"}, strings.NewReader(""), &out, &errOut); !errors.Is(err, errImageInput) || wrote {
		t.Fatalf("err = %v, clipboard written = %v", err, wrote)
	}
}

func TestImageTarget(t *testing.T) {
	fakePNG := []byte("\x89PNG fake")
	var rendered Table
	var wrapAt int
	imageRenderer = func(tb Table, w int) ([]byte, error) { rendered, wrapAt = tb, w; return fakePNG, nil }
	var copied []byte
	textCopied := false
	imageWriter = func(p []byte) error { copied = p; return nil }
	clipboardWriter = func(string, string) error { textCopied = true; return nil }
	t.Cleanup(func() {
		imageRenderer, imageWriter, clipboardWriter = renderImage, writeImageClipboard, writeClipboard
	})

	// Piped: the PNG bytes go to stdout, nothing to the clipboard.
	out, _, err := cli(t, md, "image", "--width", "30")
	if err != nil || out != string(fakePNG) || copied != nil || wrapAt != 30 || len(rendered.Rows) != 2 {
		t.Fatalf("err=%v out=%q copied=%q wrap=%d", err, out, copied, wrapAt)
	}
	// To the clipboard: only the image, and the message says so.
	_, stderr, err := cli(t, md, "image", "--copy")
	if err != nil || !bytes.Equal(copied, fakePNG) || textCopied || !strings.Contains(stderr, "copied 2 rows as an image") {
		t.Fatalf("err=%v copied=%q text=%v stderr=%q", err, copied, textCopied, stderr)
	}
	// PNG bytes never go to a terminal.
	stdoutTerminal = func() bool { return true }
	var o, e bytes.Buffer
	if err := run([]string{"image", "--stdout"}, strings.NewReader(md), &o, &e); err == nil || o.Len() != 0 {
		t.Fatalf("err=%v wrote %d bytes to a terminal", err, o.Len())
	}
}

func TestTSVWithPipeInCell(t *testing.T) {
	got, kind, err := readTable(source{Text: "a\tb | c\nd\te\n"})
	if err != nil || kind != "tsv" || !sameRows(got.Rows, [][]string{{"a", "b | c"}, {"d", "e"}}) {
		t.Fatalf("kind %s rows %q err %v", kind, got.Rows, err)
	}
}
