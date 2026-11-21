package clix

import "github.com/SCKelemen/clix/v2/internal/termesc"

// EscapeControlChars returns s with the characters a terminal acts on
// shown as visible escapes, as text output prints strings (see
// FormatData): C0 control characters other than tab and newline, DEL, C1
// control characters, the bidirectional embedding, override and isolate
// controls (U+202A to U+202E and U+2066 to U+2069) and the bytes of
// invalid UTF-8. ESC, which starts every escape sequence, prints as \x1b,
// so ESC [ 3 1 m is the text \x1b[31m; carriage return prints as \r,
// unless a newline follows it; other C0 controls and DEL print as \xHH,
// the rest as \uHHHH, and invalid bytes as \xHH. Plain ASCII and UTF-8
// text, backslashes included, is returned unchanged.
//
// Use it for strings a command prints itself that it did not write, such
// as names or messages from a server:
//
//	fmt.Fprintf(ctx.App.Out, "Deleted %s\n", clix.EscapeControlChars(name))
//
// Without it, an escape sequence in such a string could change colors,
// move the cursor and overwrite earlier output, set the window title or
// write a hyperlink whose text hides its target; a carriage return could
// overwrite the start of the line. This is the class of kubectl's
// CVE-2021-25743 (kubernetes/kubernetes#112553), whose fix escapes ESC
// and carriage return in kubectl's human-readable output.
func EscapeControlChars(s string) string { return termesc.Text(s) }
