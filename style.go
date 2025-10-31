package clix

// TextStyle describes an optional styling function applied to CLI text output.
type TextStyle interface {
	Render(string) string
}

// StyleFunc adapts a plain function into a TextStyle.
type StyleFunc func(string) string

// Render applies the style function to the provided string.
func (fn StyleFunc) Render(s string) string {
	if fn == nil {
		return s
	}
	return fn(s)
}

func renderText(style TextStyle, value string) string {
	if style == nil {
		return value
	}
	return style.Render(value)
}
