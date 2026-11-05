package main

import (
	"bytes"
	"image/png"
	"runtime"
	"testing"
)

// Renders for real with AppKit (no clipboard use).
func TestRenderImage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the image target needs macOS")
	}
	size := func(tb Table, wrapAt int) (int, int) {
		t.Helper()
		b, err := renderImage(tb, wrapAt)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		r := img.Bounds()
		return r.Dx(), r.Dy()
	}
	w, h := size(sample, 40)
	if w < 300 || h < 100 || w%2 != 0 || h%2 != 0 {
		t.Fatalf("image %dx%d: want a 2x render of a 3-column table", w, h)
	}
	// Wrapping narrower makes the picture narrower and taller.
	w2, h2 := size(sample, 15)
	if w2 >= w || h2 <= h {
		t.Fatalf("width 15: %dx%d, width 40: %dx%d", w2, h2, w, h)
	}
	// A table with no header renders too.
	if w3, _ := size(Table{Rows: [][]string{{"a", "b"}}}, 0); w3 == 0 {
		t.Fatal("empty image")
	}
}
