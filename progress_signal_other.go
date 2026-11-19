//go:build !unix

package clix

// watchProgressSignals does nothing outside Unix, where a signal cannot be
// delivered again after it was caught. Indicators rely on their context
// there: App.Main cancels it on Ctrl-C.
func watchProgressSignals(restore func()) (stop func()) {
	return func() {}
}
