package clix

// TextStyle describes an optional styling function applied to CLI text output.
// This interface is compatible with github.com/charmbracelet/lipgloss.Style,
// allowing lipgloss styles to be used directly without wrapping.
type TextStyle interface {
	Render(...string) string
}

// StyleFunc adapts a plain function into a TextStyle.
// For compatibility with lipgloss.Style, the function accepts variadic strings.
// When called with a single string, it behaves as expected.
type StyleFunc func(...string) string

// Render applies the style function to the provided strings.
func (fn StyleFunc) Render(strs ...string) string {
	if fn == nil {
		if len(strs) == 0 {
			return ""
		}
		return strs[0]
	}
	return fn(strs...)
}

func renderText(style TextStyle, value string) string {
	if style == nil {
		return value
	}
	return style.Render(value)
}
