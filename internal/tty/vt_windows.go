//go:build windows

package tty

import (
	"os"

	"golang.org/x/sys/windows"
)

const defaultUTF8 = true

// supportsVirtualTerminal turns on virtual terminal processing for the
// console behind w, so ANSI escapes are interpreted instead of printed. It
// fails on legacy consoles (before Windows 10 1511), which then get plain
// text. The mode is left on, as other console programs do.
//
// https://learn.microsoft.com/windows/console/console-virtual-terminal-sequences
func supportsVirtualTerminal(w any) bool {
	fd, ok := Fd(w)
	if !ok {
		return false
	}
	h := windows.Handle(fd)
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}

// EnableVirtualTerminal prepares a console for a raw-mode prompt: virtual
// terminal processing on out (so cursor movement works) and virtual
// terminal input on in (so arrow and function keys arrive as escape
// sequences). The returned function restores both modes. An error means the
// console cannot do this (legacy conhost) and the caller should fall back to
// line-based prompts.
func EnableVirtualTerminal(in, out *os.File) (restore func(), err error) {
	hin, hout := windows.Handle(in.Fd()), windows.Handle(out.Fd())
	var inMode, outMode uint32
	if err := windows.GetConsoleMode(hout, &outMode); err != nil {
		return nil, err
	}
	if err := windows.GetConsoleMode(hin, &inMode); err != nil {
		return nil, err
	}
	if err := windows.SetConsoleMode(hout, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nil, err
	}
	if err := windows.SetConsoleMode(hin, inMode|windows.ENABLE_VIRTUAL_TERMINAL_INPUT); err != nil {
		_ = windows.SetConsoleMode(hout, outMode)
		return nil, err
	}
	return func() {
		_ = windows.SetConsoleMode(hin, inMode)
		_ = windows.SetConsoleMode(hout, outMode)
	}, nil
}

// DisableEcho turns off echo on the console behind f while keeping line
// editing, for reading a secret a line at a time. The returned function
// restores the previous mode.
func DisableEcho(f *os.File) (restore func(), err error) {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return nil, err
	}
	newMode := (mode &^ windows.ENABLE_ECHO_INPUT) | windows.ENABLE_PROCESSED_INPUT | windows.ENABLE_LINE_INPUT
	if err := windows.SetConsoleMode(h, newMode); err != nil {
		return nil, err
	}
	return func() { _ = windows.SetConsoleMode(h, mode) }, nil
}
