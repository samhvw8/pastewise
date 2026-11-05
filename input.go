package main

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var errImageInput = errors.New("the clipboard holds an image; pastewise cannot read a table from an image yet")

// source is what the clipboard holds: plain text, HTML (from the HTML type, or
// converted from an RTF table), and whether an image is on it.
type source struct {
	Text, HTML string
	Image      bool
}

// readTable finds the table in the input and says which kind it was, for the
// usage log. A rich-text table goes first because it keeps the header, which
// tab-separated text loses. Every text target pastewise writes reads back in, so
// any output converts to any other.
func readTable(in source) (Table, string, error) {
	// HTML text (a saved page on stdin) is read as HTML before a "|" in it
	// can pass for a pipe table.
	rich := []string{in.HTML}
	if strings.HasPrefix(strings.TrimSpace(in.Text), "<") {
		rich = append(rich, in.Text)
	}
	for _, h := range rich {
		if hasHTMLTable(h) {
			if t, err := ParseHTML(h); err == nil {
				return t, "html", nil
			}
		}
	}
	// Parse takes any line with a "|", so the stricter shapes go first: a list
	// (a title, then only bullets), and tab-separated text when more lines have
	// tabs than pipes (a "|" inside a spreadsheet cell).
	if t, err := ParseList(in.Text); err == nil {
		return t, "list", nil
	}
	tabs, pipes := 0, 0
	for _, l := range strings.Split(in.Text, "\n") {
		if strings.Contains(l, "\t") {
			tabs++
		}
		if strings.ContainsAny(l, "|│┃║") {
			pipes++
		}
	}
	if tabs > pipes {
		if t, err := ParseTSV(in.Text); err == nil {
			return t, "tsv", nil
		}
	}
	if t, err := Parse(in.Text); err == nil {
		return t, inputKind(in.Text), nil
	}
	if t, err := ParseTSV(in.Text); err == nil {
		return t, "tsv", nil
	}
	if in.Image {
		return Table{}, "image", errImageInput
	}
	return Table{}, "none", errNoTable
}

var tableTag = regexp.MustCompile(`(?i)<table[\s>]`)

func hasHTMLTable(s string) bool { return tableTag.MatchString(s) }

// padRows makes every row as wide as the widest one.
func padRows(rows [][]string) [][]string {
	ncol := 0
	for _, r := range rows {
		ncol = max(ncol, len(r))
	}
	for i, r := range rows {
		for len(r) < ncol {
			r = append(r, "")
		}
		rows[i] = r
	}
	return rows
}

// ParseTSV reads tab-separated cells, the plain text that Apple Notes, Numbers,
// Excel and Google Sheets put on the clipboard. It has no header: plain text
// does not say which row is one. Lines with no tab are text around the table
// (a Notes copy starts with the note's title), so they are skipped; spreadsheets
// keep the tabs of empty cells.
func ParseTSV(text string) (Table, error) {
	var rows [][]string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if !strings.Contains(line, "\t") || strings.TrimSpace(line) == "" {
			continue
		}
		cells := strings.Split(line, "\t")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	rows = padRows(rows)
	if len(rows) == 0 || len(rows[0]) < 2 {
		return Table{}, errNoTable
	}
	return Table{Rows: rows}, nil
}

// ParseList reads the list target back: blocks split by blank lines, a title
// line (bold markers removed), then "• " bullets. When every bullet is
// "Key: value", the keys become the header and the title column's name is
// empty, because the list never shows it.
func ParseList(text string) (Table, error) {
	type block struct {
		title   string
		bullets []string
	}
	var blocks []block
	var cur *block
	bulletCount := 0
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			cur = nil
			continue
		}
		b, isBullet := cutBullet(line)
		if cur == nil {
			if isBullet {
				return Table{}, errNoTable // a list block starts with its title
			}
			blocks = append(blocks, block{title: stripBold(line)})
			cur = &blocks[len(blocks)-1]
			continue
		}
		if !isBullet {
			return Table{}, errNoTable
		}
		cur.bullets = append(cur.bullets, b)
		bulletCount++
	}
	if bulletCount == 0 {
		return Table{}, errNoTable
	}

	keyed := true
	for _, b := range blocks {
		for _, s := range b.bullets {
			if k, _, ok := strings.Cut(s, ": "); !ok || strings.TrimSpace(k) == "" {
				keyed = false
			}
		}
	}
	if !keyed {
		rows := make([][]string, len(blocks))
		for i, b := range blocks {
			rows[i] = append([]string{b.title}, b.bullets...)
		}
		return Table{Rows: padRows(rows)}, nil
	}
	header := []string{""}
	col := map[string]int{}
	for _, b := range blocks {
		for _, s := range b.bullets {
			k, _, _ := strings.Cut(s, ": ")
			if _, ok := col[k]; !ok {
				col[k] = len(header)
				header = append(header, k)
			}
		}
	}
	rows := make([][]string, len(blocks))
	for i, b := range blocks {
		row := make([]string, len(header))
		row[0] = b.title
		for _, s := range b.bullets {
			k, v, _ := strings.Cut(s, ": ")
			row[col[k]] = strings.TrimSpace(strings.Join(strings.Fields(row[col[k]]+" "+v), " "))
		}
		rows[i] = row
	}
	return Table{Header: header, Rows: rows}, nil
}

func cutBullet(line string) (string, bool) {
	for _, p := range []string{"• ", "- ", "* "} {
		if rest, ok := strings.CutPrefix(line, p); ok {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

func stripBold(s string) string {
	for _, m := range []string{"**", "__", "*", "_"} {
		if len(s) > 2*len(m) && strings.HasPrefix(s, m) && strings.HasSuffix(s, m) {
			return strings.TrimSpace(s[len(m) : len(s)-len(m)])
		}
	}
	return s
}

// ParseHTML reads the first <table> in an HTML document: from pastewise's own
// notes target, Apple Notes (via RTF), Google Docs and Sheets, Excel, Numbers
// or a web page. A cell's text is joined onto one line. colspan and rowspan
// leave empty cells, so the columns stay in line. The first row is the header
// when it is in <thead>, is all <th>, or is all bold while a later row is not.
func ParseHTML(s string) (Table, error) {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return Table{}, err
	}
	tbl := findFirst(doc, atom.Table)
	if tbl == nil {
		return Table{}, errNoTable
	}

	type row struct {
		cells    []string
		head     bool // in <thead> or all <th>
		allBold  bool
		hasCells bool
	}
	var rows []row
	var carry []int // per column: rows still covered by a rowspan above
	var walk func(n *html.Node, inHead bool)
	walk = func(n *html.Node, inHead bool) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.DataAtom {
			case atom.Table:
				continue // a nested table belongs to its cell
			case atom.Thead:
				walk(c, true)
			case atom.Tr:
				r := row{head: inHead, allBold: true}
				allTH := true
				col := 0
				skipCarried := func() {
					for col < len(carry) && carry[col] > 0 {
						carry[col]--
						r.cells = append(r.cells, "")
						col++
					}
				}
				for td := c.FirstChild; td != nil; td = td.NextSibling {
					if td.Type != html.ElementNode || (td.DataAtom != atom.Td && td.DataAtom != atom.Th) {
						continue
					}
					skipCarried()
					text := cellText(td)
					if td.DataAtom != atom.Th {
						allTH = false
					}
					if text != "" && td.DataAtom != atom.Th && !allTextBold(td, td) {
						r.allBold = false
					}
					span, down := spanAttr(td, "colspan"), spanAttr(td, "rowspan")
					for k := 0; k < span; k++ {
						if k == 0 {
							r.cells = append(r.cells, text)
						} else {
							r.cells = append(r.cells, "")
						}
						for len(carry) <= col {
							carry = append(carry, 0)
						}
						carry[col] = down - 1
						col++
					}
					r.hasCells = true
				}
				for ; col < len(carry); col++ { // trailing columns a rowspan covers
					if carry[col] > 0 {
						carry[col]--
					}
					r.cells = append(r.cells, "")
				}
				if !r.hasCells {
					continue
				}
				r.head = r.head || allTH
				empty := true
				for _, x := range r.cells {
					if x != "" {
						empty = false
					}
				}
				if !empty {
					rows = append(rows, r)
				}
			default:
				walk(c, inHead)
			}
		}
	}
	walk(tbl, false)
	if len(rows) == 0 {
		return Table{}, errNoTable
	}

	cells := make([][]string, len(rows))
	for i, r := range rows {
		cells[i] = r.cells
	}
	cells = padRows(cells)
	t := Table{Rows: cells}
	if len(rows) > 1 {
		laterPlain := false
		for _, r := range rows[1:] {
			if !r.allBold {
				laterPlain = true
			}
		}
		if rows[0].head || (rows[0].allBold && laterPlain) {
			t.Header, t.Rows = cells[0], cells[1:]
		}
	}
	return t, nil
}

func findFirst(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := findFirst(c, a); f != nil {
			return f
		}
	}
	return nil
}

// cellText is the cell's text on one line: block breaks and <br> become spaces.
func cellText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			b.WriteString(n.Data)
		case html.ElementNode:
			switch n.DataAtom {
			case atom.Style, atom.Script:
				return
			case atom.Br, atom.P, atom.Div, atom.Li, atom.Tr, atom.Td, atom.Th:
				b.WriteString(" ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && (n.DataAtom == atom.P || n.DataAtom == atom.Div || n.DataAtom == atom.Li) {
			b.WriteString(" ")
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

var boldStyle = regexp.MustCompile(`(?i)font-weight\s*:\s*(bold|bolder|[6-9]00)`)

func isBoldElement(n *html.Node) bool {
	if n.DataAtom == atom.B || n.DataAtom == atom.Strong || n.DataAtom == atom.Th {
		return true
	}
	for _, a := range n.Attr {
		if a.Key == "style" && boldStyle.MatchString(a.Val) {
			return true
		}
	}
	return false
}

// allTextBold reports whether every non-blank text node under n has a bold
// element between it and the cell (or is in a bold cell).
func allTextBold(n, cell *html.Node) bool {
	bold := false
	for p := n; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && isBoldElement(p) {
			bold = true
			break
		}
		if p == cell {
			break
		}
	}
	if n.Type == html.TextNode {
		return bold || strings.TrimSpace(n.Data) == ""
	}
	if bold {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !allTextBold(c, cell) {
			return false
		}
	}
	return true
}

func spanAttr(n *html.Node, key string) int {
	for _, a := range n.Attr {
		if a.Key == key {
			if v, err := strconv.Atoi(strings.TrimSpace(a.Val)); err == nil && v > 1 {
				return min(v, 1000)
			}
		}
	}
	return 1
}
