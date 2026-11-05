package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"unicode/utf16"
)

var errNoClipboard = errors.New("the clipboard needs macOS; pipe the table in instead (pastewise md < table.txt)")

// macCommand runs a macOS tool by absolute path with a UTF-8 locale. Alfred runs
// scripts with no PATH guarantee and no LANG, and without a locale pbpaste
// turns every non-ASCII character (such as "│") into "?".
func macCommand(name string, args ...string) *exec.Cmd {
	cmd := exec.Command("/usr/bin/"+name, args...)
	cmd.Env = append(os.Environ(), "LANG=en_US.UTF-8", "LC_ALL=en_US.UTF-8")
	return cmd
}

// readJXA saves the clipboard's plain text, HTML and RTF as raw bytes into dir,
// so no text passes through a locale, and prints which types it found.
const readJXA = `
ObjC.import('AppKit');
function run(argv) {
  var dir = argv[0], pb = $.NSPasteboard.generalPasteboard, got = [];
  function save(type, name) {
    if (!pb.types.containsObject(type)) return;
    var d = pb.dataForType(type);
    if (!d.isNil() && d.writeToFileAtomically($(dir + '/' + name), true)) got.push(name);
  }
  save('public.utf8-plain-text', 'text');
  save('public.html', 'html');
  save('public.rtf', 'rtf');
  if (pb.types.containsObject('public.png') || pb.types.containsObject('public.tiff')) got.push('image');
  return got.join(' ');
}
`

// readClipboard reads the plain text, the HTML and whether an image is there.
// An RTF table (Apple Notes puts only RTF) becomes HTML through textutil, the
// same converter the notes target uses the other way.
func readClipboard() (source, error) {
	if runtime.GOOS != "darwin" {
		return source{}, errNoClipboard
	}
	dir, err := os.MkdirTemp("", "pastewise-read-*")
	if err != nil {
		return source{}, err
	}
	defer os.RemoveAll(dir)
	cmd := macCommand("osascript", "-l", "JavaScript", "-e", readJXA, dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return source{}, fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	got := strings.Fields(string(out))
	var c source
	c.Image = slices.Contains(got, "image")
	if slices.Contains(got, "text") {
		b, err := os.ReadFile(filepath.Join(dir, "text"))
		if err != nil {
			return source{}, err
		}
		c.Text = string(b)
	}
	if slices.Contains(got, "html") {
		b, err := os.ReadFile(filepath.Join(dir, "html"))
		if err != nil {
			return source{}, err
		}
		c.HTML = decodeHTML(b)
	}
	if !hasHTMLTable(c.HTML) && slices.Contains(got, "rtf") {
		rtfPath := filepath.Join(dir, "rtf")
		b, err := os.ReadFile(rtfPath)
		if err != nil {
			return source{}, err
		}
		if bytes.Contains(b, []byte(`\trowd`)) { // RTF has a table
			h, err := macCommand("textutil", "-convert", "html", "-format", "rtf", "-stdout", rtfPath).Output()
			if err != nil {
				return source{}, fmt.Errorf("textutil: %w", err)
			}
			c.HTML = string(h)
		}
	}
	return c, nil
}

// decodeHTML turns clipboard HTML bytes into a string. Some apps write UTF-16
// with a byte-order mark; the rest write UTF-8.
func decodeHTML(b []byte) string {
	var order binary.ByteOrder
	switch {
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		order = binary.LittleEndian
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		order = binary.BigEndian
	default:
		return string(bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}))
	}
	u := make([]uint16, (len(b)-2)/2)
	for i := range u {
		u[i] = order.Uint16(b[2+2*i:])
	}
	return string(utf16.Decode(u))
}

// The JXA script clears the pasteboard and sets every type in one go, so each
// app takes the richest type it understands. argv: plain, html, rtf-path.
const pasteboardJXA = `
ObjC.import('AppKit');
function run(argv) {
  var pb = $.NSPasteboard.generalPasteboard;
  pb.clearContents;
  if (argv[1]) pb.setStringForType($(argv[1]), 'public.html');
  if (argv[2]) pb.setDataForType($.NSData.dataWithContentsOfFile($(argv[2])), 'public.rtf');
  pb.setStringForType($(argv[0]), 'public.utf8-plain-text');
}
`

// writeClipboard puts plain text on the clipboard, plus HTML and RTF when richHTML is set.
func writeClipboard(plain, richHTML string) error {
	if runtime.GOOS != "darwin" {
		return errNoClipboard
	}
	rtfPath := ""
	if richHTML != "" {
		cmd := macCommand("textutil", "-stdin", "-format", "html", "-inputencoding", "UTF-8", "-convert", "rtf", "-stdout")
		cmd.Stdin = strings.NewReader(richHTML)
		rtf, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("textutil: %w", err)
		}
		f, err := os.CreateTemp("", "pastewise-*.rtf")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		if _, err := f.Write(rtf); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		rtfPath = f.Name()
	}
	cmd := macCommand("osascript", "-l", "JavaScript", "-e", pasteboardJXA, plain, richHTML, rtfPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
