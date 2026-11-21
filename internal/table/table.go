// Package table writes rows of text the way gh does: as an aligned table
// with a header on a terminal, and as tab-separated values without a
// header everywhere else, so that scripts read every value in full.
package table

import (
	"io"
	"strings"

	"github.com/SCKelemen/clix/v2/internal/termesc"
	"github.com/SCKelemen/clix/v2/internal/tty"
)

// Options configure Write.
type Options struct {
	// Terminal selects the aligned table, with a header row, for a person
	// at a terminal. Otherwise Write prints tab-separated values without a
	// header (see Write).
	Terminal bool

	// Width is the terminal's width in columns. When the table is wider,
	// the widest columns are truncated to fit. 0 means no limit.
	Width int

	// Ellipsis marks a truncated cell, such as "…" or "...".
	Ellipsis string

	// Header styles the header row of a terminal table; nil leaves it
	// plain.
	Header func(string) string

	// RawControl writes the control characters of tab-separated cells as
	// they are, with only the linear TSV escapes (see Write). A terminal
	// table escapes them regardless, since it lays cells out by their
	// display width.
	RawControl bool
}

// Separator is the space between two columns of a terminal table.
const Separator = "  "

// Write prints header and rows to w.
//
// On a terminal (Options.Terminal) the header comes first, the columns are
// aligned by display width and separated by two spaces, the last column is
// not padded, and control characters in a cell are shown escaped ("\n",
// "\t", "\x1b"; see termesc.Line), so a value can neither break the layout
// nor send escape sequences to the terminal.
//
// Otherwise every row is one line of tab-separated values, with no header,
// no padding and no truncation, in the linear TSV convention: a backslash,
// tab, newline or carriage return in a cell is written as \\, \t, \n or
// \r, so every line is one row and every tab separates two cells. The other
// control characters are escaped as on a terminal ("\x1b"), since TSV
// often reaches one through grep or less; with the backslash escaped, the
// escapes stay unambiguous.
// https://www.iana.org/assignments/media-types/text/tab-separated-values
func Write(w io.Writer, header []string, rows [][]string, opts Options) error {
	if !opts.Terminal {
		return writeTSV(w, rows, opts.RawControl)
	}
	all := make([][]string, 0, len(rows)+1)
	if header != nil {
		all = append(all, header)
	}
	all = append(all, rows...)
	for r, row := range all {
		cells := make([]string, len(row))
		for i, c := range row {
			cells[i] = termesc.Line(c)
		}
		all[r] = cells
	}
	widths := columnWidths(all, opts.Width)
	var b strings.Builder
	for r, row := range all {
		b.Reset()
		for col := range widths {
			cell := ""
			if col < len(row) {
				cell = row[col]
			}
			cell = Truncate(cell, widths[col], opts.Ellipsis)
			last := col == len(widths)-1
			pad := widths[col] - tty.StringWidth(cell)
			if r == 0 && header != nil && opts.Header != nil && cell != "" {
				cell = opts.Header(cell)
			}
			b.WriteString(cell)
			if !last {
				if pad > 0 {
					b.WriteString(strings.Repeat(" ", pad))
				}
				b.WriteString(Separator)
			}
		}
		line := strings.TrimRight(b.String(), " ")
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeTSV(w io.Writer, rows [][]string, raw bool) error {
	var b strings.Builder
	for _, row := range rows {
		b.Reset()
		for i, cell := range row {
			if i > 0 {
				b.WriteByte('\t')
			}
			cell = EscapeTSV(cell)
			if !raw {
				cell = termesc.Text(cell)
			}
			b.WriteString(cell)
		}
		b.WriteByte('\n')
		if _, err := io.WriteString(w, b.String()); err != nil {
			return err
		}
	}
	return nil
}

// tsvEscaper implements the linear TSV escapes.
var tsvEscaper = strings.NewReplacer(`\`, `\\`, "\t", `\t`, "\n", `\n`, "\r", `\r`)

// EscapeTSV escapes a cell for tab-separated output: a backslash, tab,
// newline or carriage return becomes \\, \t, \n or \r.
func EscapeTSV(s string) string {
	if !strings.ContainsAny(s, "\\\t\n\r") {
		return s
	}
	return tsvEscaper.Replace(s)
}

// Truncate shortens s to at most width display columns, ending it with
// ellipsis when anything was cut. It cuts between grapheme clusters, so a
// wide character or an emoji sequence is never split. When width cannot
// hold the ellipsis, s is cut without one.
func Truncate(s string, width int, ellipsis string) string {
	if width < 0 {
		width = 0
	}
	if tty.StringWidth(s) <= width {
		return s
	}
	ew := tty.StringWidth(ellipsis)
	if ew > width {
		ellipsis, ew = "", 0
	}
	limit := width - ew
	var b strings.Builder
	used := 0
	for rest := s; rest != ""; {
		cluster, cw := tty.FirstGrapheme(rest)
		if used+cw > limit {
			break
		}
		b.WriteString(cluster)
		used += cw
		rest = rest[len(cluster):]
	}
	return b.String() + ellipsis
}

// columnWidths returns the display width of each column: the width of its
// widest cell, shrunk so that the table fits in maxWidth columns when
// maxWidth > 0. Columns narrower than an even share of the width keep
// their width, the others share what is left, and any width still free
// goes back to the truncated columns from the left. This is the layout of
// gh's table printer.
func columnWidths(rows [][]string, maxWidth int) []int {
	numCols := 0
	for _, row := range rows {
		numCols = max(numCols, len(row))
	}
	natural := make([]int, numCols)
	for _, row := range rows {
		for col, cell := range row {
			natural[col] = max(natural[col], tty.StringWidth(cell))
		}
	}
	total := 0
	for _, w := range natural {
		total += w
	}
	if maxWidth <= 0 || numCols == 0 || total+len(Separator)*(numCols-1) <= maxWidth {
		return natural
	}

	widths := make([]int, numCols)
	avail := func() int {
		used := 0
		for _, w := range widths {
			used += w
		}
		return maxWidth - len(Separator)*(numCols-1) - used
	}
	flexible := func() int {
		n := 0
		for _, w := range widths {
			if w == 0 {
				n++
			}
		}
		return n
	}
	// Columns narrower than an even share keep their natural width.
	if a, n := avail(), flexible(); a > 0 && n > 0 {
		share := a / n
		for col, w := range natural {
			if w < share {
				widths[col] = w
			}
		}
	}
	// The others share what is left.
	if n := flexible(); n > 0 {
		share := avail() / n
		for col, w := range natural {
			if widths[col] != 0 {
				continue
			}
			if w < share {
				widths[col] = w
			} else if share > 0 {
				widths[col] = share
			}
		}
	}
	// Width still free goes to the truncated columns, from the left.
	if a := avail(); a > 0 {
		for col := range widths {
			add := min(natural[col]-widths[col], a)
			widths[col] += add
			a -= add
			if a <= 0 {
				break
			}
		}
	}
	return widths
}
