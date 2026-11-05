package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed image.js
var imageJXA string

// pointsPerColumn turns --width (text columns) into a cell width in points
// for the 14 pt system font.
const pointsPerColumn = 8

// renderImage draws the table as a PNG. wrapAt is --width; 0 means no wrap.
func renderImage(t Table, wrapAt int) ([]byte, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("the image target needs macOS")
	}
	dir, err := os.MkdirTemp("", "pastewise-image-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	spec, err := json.Marshal(map[string]any{"header": t.Header, "rows": t.Rows, "maxWidth": wrapAt * pointsPerColumn})
	if err != nil {
		return nil, err
	}
	specPath, pngPath := filepath.Join(dir, "spec.json"), filepath.Join(dir, "table.png")
	if err := os.WriteFile(specPath, spec, 0o600); err != nil {
		return nil, err
	}
	cmd := macCommand("osascript", "-l", "JavaScript", "-e", imageJXA, specPath, pngPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return os.ReadFile(pngPath)
}

// writeImageJXA puts the PNG on the clipboard as PNG and TIFF, and no text:
// with text there, apps such as Slack paste the text instead of the picture.
const writeImageJXA = `
ObjC.import('AppKit');
function run(argv) {
  var d = $.NSData.dataWithContentsOfFile($(argv[0]));
  var img = $.NSImage.alloc.initWithData(d);
  if (img.isNil()) throw new Error('not an image: ' + argv[0]);
  var pb = $.NSPasteboard.generalPasteboard;
  pb.clearContents;
  pb.setDataForType(d, 'public.png');
  pb.setDataForType(img.TIFFRepresentation, 'public.tiff');
}
`

func writeImageClipboard(png []byte) error {
	if runtime.GOOS != "darwin" {
		return errNoClipboard
	}
	f, err := os.CreateTemp("", "pastewise-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(png); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	cmd := macCommand("osascript", "-l", "JavaScript", "-e", writeImageJXA, f.Name())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
