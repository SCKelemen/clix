package ptytest

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/SCKelemen/clix/v2/internal/tty"
)

// Screen replays terminal output on an emulated rows x cols screen and
// returns the visible lines, right-trimmed. It implements what prompts
// use: printable text with auto-wrap (including the deferred wrap at the
// last column), wide characters, CR, LF (scrolling at the bottom), BS, and
// the CSI sequences CUU/CUD/CUF/CUB (A, B, C, D), CHA (G), ED (J) and EL (K).
// Other escape sequences (colors, modes) are ignored. It is meant for
// asserting what a person would see after a prompt has redrawn itself.
func Screen(output string, rows, cols int) []string {
	s := newScreen(rows, cols)
	s.feed(output)
	lines := make([]string, rows)
	for i, row := range s.cells {
		lines[i] = strings.TrimRight(strings.Join(row, ""), " ")
	}
	return lines
}

// ScreenText is Screen joined with newlines, with trailing empty lines
// removed.
func ScreenText(output string, rows, cols int) string {
	lines := Screen(output, rows, cols)
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

type screen struct {
	rows, cols int
	cells      [][]string
	row, col   int
	wrapNext   bool
}

func newScreen(rows, cols int) *screen {
	s := &screen{rows: rows, cols: cols}
	for i := 0; i < rows; i++ {
		s.cells = append(s.cells, s.blank())
	}
	return s
}

func (s *screen) blank() []string {
	row := make([]string, s.cols)
	for i := range row {
		row[i] = " "
	}
	return row
}

func (s *screen) lineFeed() {
	if s.row == s.rows-1 {
		s.cells = append(s.cells[1:], s.blank())
	} else {
		s.row++
	}
}

func (s *screen) put(r rune) {
	w := tty.RuneWidth(r)
	if w == 0 {
		return
	}
	if s.wrapNext || s.col+w > s.cols {
		s.col = 0
		s.lineFeed()
		s.wrapNext = false
	}
	s.cells[s.row][s.col] = string(r)
	if w == 2 && s.col+1 < s.cols {
		s.cells[s.row][s.col+1] = ""
	}
	s.col += w
	if s.col >= s.cols {
		s.col = s.cols - 1
		s.wrapNext = true
	}
}

func (s *screen) clamp() {
	if s.row < 0 {
		s.row = 0
	}
	if s.row >= s.rows {
		s.row = s.rows - 1
	}
	if s.col < 0 {
		s.col = 0
	}
	if s.col >= s.cols {
		s.col = s.cols - 1
	}
}

func (s *screen) feed(out string) {
	for i := 0; i < len(out); {
		c := out[i]
		switch {
		case c == 0x1b:
			i = s.escape(out, i)
			continue
		case c == '\r':
			s.col, s.wrapNext = 0, false
		case c == '\n':
			s.lineFeed()
			s.wrapNext = false
		case c == '\b':
			if s.col > 0 {
				s.col--
			}
			s.wrapNext = false
		case c < 0x20 || c == 0x7f:
			// other control characters do nothing visible
		default:
			r, size := utf8.DecodeRuneInString(out[i:])
			s.put(r)
			i += size
			continue
		}
		i++
	}
}

// escape handles the sequence starting at out[i] and returns the index
// after it.
func (s *screen) escape(out string, i int) int {
	if i+1 >= len(out) {
		return len(out)
	}
	if out[i+1] != '[' {
		if out[i+1] == ']' { // OSC: skip to BEL or ST
			for j := i + 2; j < len(out); j++ {
				if out[j] == 0x07 {
					return j + 1
				}
				if out[j] == 0x1b && j+1 < len(out) && out[j+1] == '\\' {
					return j + 2
				}
			}
			return len(out)
		}
		return i + 2
	}
	j := i + 2
	for j < len(out) && out[j] >= 0x20 && out[j] <= 0x3f {
		j++
	}
	if j >= len(out) {
		return len(out)
	}
	params, final := out[i+2:j], out[j]
	n := 1
	if p, err := strconv.Atoi(params); err == nil && p > 0 {
		n = p
	}
	private := strings.HasPrefix(params, "?")
	switch {
	case private:
	case final == 'A':
		s.row -= n
		s.wrapNext = false
	case final == 'B':
		s.row += n
		s.wrapNext = false
	case final == 'C':
		s.col += n
		s.wrapNext = false
	case final == 'D':
		s.col -= n
		s.wrapNext = false
	case final == 'G':
		s.col = n - 1
		s.wrapNext = false
	case final == 'J':
		mode, _ := strconv.Atoi(params)
		if mode == 0 {
			for c := s.col; c < s.cols; c++ {
				s.cells[s.row][c] = " "
			}
			for r := s.row + 1; r < s.rows; r++ {
				s.cells[r] = s.blank()
			}
		}
	case final == 'K':
		mode, _ := strconv.Atoi(params)
		if mode == 0 {
			for c := s.col; c < s.cols; c++ {
				s.cells[s.row][c] = " "
			}
		}
	}
	s.clamp()
	return j + 1
}
