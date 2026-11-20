package clix

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Logging, like gcloud's: the app logs with the *slog.Logger that
// App.Logger (or Context.Logger) returns, which writes to App.Err, and the
// person running the program chooses how much to see with the global
// --verbosity flag that EnableVerbosity registers.

// The levels --verbosity names, as slog levels. debug, info, warning and
// error are slog.LevelDebug, slog.LevelInfo, slog.LevelWarn and
// slog.LevelError.
const (
	// LevelCritical is gcloud's critical level, above slog.LevelError. Log
	// at it with Logger().Log(ctx, clix.LevelCritical, msg).
	LevelCritical slog.Level = 12

	// LevelNone is the level of --verbosity=none: above every level a
	// program logs at, so nothing is written.
	LevelNone slog.Level = math.MaxInt32
)

const verbosityFlagName = "verbosity"

// defaultVerbosity is the --verbosity default, as in gcloud.
const defaultVerbosity = "warning"

// verbosityLevels lists the values --verbosity accepts, from the most to
// the least output.
var verbosityLevels = []struct {
	name  string
	level slog.Level
}{
	{"debug", slog.LevelDebug},
	{"info", slog.LevelInfo},
	{"warning", slog.LevelWarn},
	{"error", slog.LevelError},
	{"critical", LevelCritical},
	{"none", LevelNone},
}

// verbosityNames returns the names of the levels, in order.
func verbosityNames() []string {
	names := make([]string, len(verbosityLevels))
	for i, l := range verbosityLevels {
		names[i] = l.name
	}
	return names
}

// parseVerbosity returns the level a --verbosity value names, ignoring
// ASCII case, and its canonical (lower-case) name.
func parseVerbosity(s string) (name string, level slog.Level, err error) {
	for _, l := range verbosityLevels {
		if strings.EqualFold(s, l.name) {
			return l.name, l.level, nil
		}
	}
	return "", 0, fmt.Errorf("unknown verbosity %q (valid levels: %s)", s, strings.Join(verbosityNames(), ", "))
}

// verbosityValue is the Value of the --verbosity flag that EnableVerbosity
// registers. Its type tells that flag apart from an app flag of the same
// name, which wins and is then not a log level.
type verbosityValue struct{ s string }

func (v *verbosityValue) Set(s string) error {
	name, _, err := parseVerbosity(s)
	if err != nil {
		return err
	}
	v.s = name
	return nil
}

func (v *verbosityValue) String() string { return v.s }

// Reset lets one App run more than once (see FlagSet.Var).
func (v *verbosityValue) Reset() { v.s = "" }

// EnableVerbosity registers gcloud's global --verbosity flag on the root,
// which sets the level of the logger that App.Logger and Context.Logger
// return:
//
//	--verbosity=debug|info|warning|error|critical|none
//
// The default is warning. A value is read from the command line, then the
// <EnvPrefix>_VERBOSITY environment variable, then the "verbosity" config
// key (both also when App.AutoEnv or App.AutoConfig is off), then the
// default. Case is ignored; any other value is a usage error that lists
// the levels, wherever it comes from. Shell completion offers the levels.
//
// The flag is Ambient (see FlagOptions.Ambient): it says how much to log,
// not what to do, so "myapp --verbosity debug widgets create" still
// prompts for missing required flags when the app is interactive.
//
// An app flag named --verbosity wins over it, as app flags win over
// extensions'; the logger then stays at warning. Calling EnableVerbosity
// again does nothing.
func (a *App) EnableVerbosity() {
	if a.verbosity {
		return
	}
	a.verbosity = true
	fs := a.Flags()
	fs.Var(VarOptions{
		FlagOptions: FlagOptions{
			Name:      verbosityFlagName,
			Usage:     fmt.Sprintf("Log verbosity: %s (default %q)", strings.Join(verbosityNames(), ", "), defaultVerbosity),
			EnvVar:    a.envVarName(verbosityFlagName),
			ConfigKey: verbosityFlagName,
			// How much to log, not an input: passing it does not stop Run
			// from prompting for missing required flags.
			Ambient: true,
		},
		Default: defaultVerbosity,
		Value:   &verbosityValue{},
	})
	if a.verbosityFlag() != nil {
		fs.SetCompletion(verbosityFlagName, CompleteValues(verbosityNames()...))
	}
}

// verbosityFlag returns the --verbosity flag EnableVerbosity registered,
// unless an app flag of that name replaced it.
func (a *App) verbosityFlag() *Flag {
	f := a.rootFlagSet().lookup(verbosityFlagName)
	if f == nil {
		return nil
	}
	if _, ours := f.Value.(*verbosityValue); !ours {
		return nil
	}
	return f
}

// beginLogLevel prepares the log level for a Run. A Run started from
// outside the App starts at the default, warning; a Run started by a
// handler keeps the level of the Run that started it and puts it back when
// it ends. Call it before startRun.
func (a *App) beginLogLevel() (end func()) {
	lv := &a.logs().level
	if a.running {
		outer := lv.Level()
		return func() { lv.Set(outer) }
	}
	lv.Set(slog.LevelWarn)
	return func() {}
}

// applyVerbosity sets the log level from --verbosity, once Run has filled
// the flags from the command line, the environment, the config file and
// the defaults.
func (a *App) applyVerbosity() {
	level := slog.LevelWarn
	if f := a.verbosityFlag(); f != nil {
		if _, l, err := parseVerbosity(f.Value.String()); err == nil {
			level = l
		}
	}
	a.logs().level.Set(level)
}

// logState is the App's logger and its level.
type logState struct {
	once   sync.Once
	level  slog.LevelVar
	logger *slog.Logger
}

// logs returns the App's logState, creating the logger the first time.
func (a *App) logs() *logState {
	s := &a.logging
	s.once.Do(func() {
		s.level.Set(slog.LevelWarn)
		s.logger = slog.New(newLogHandler(a.stderr, &s.level))
	})
	return s
}

// Logger returns the app's logger, for messages about what the program is
// doing, such as gcloud's "WARNING: ..." and "DEBUG: ..." lines:
//
//	ctx.App.Logger().Debug("fetched page", "project", project, "items", n)
//	ctx.App.Logger().Warn("quota almost used", "used", used, "limit", limit)
//
// It writes to App.Err (as it is when a record is written, os.Stderr when
// nil), never to App.Out, one line per record:
//
//	WARNING: quota almost used used=95 limit=100
//
// The level name (DEBUG, INFO, WARNING, ERROR or CRITICAL; other levels as
// slog names them, such as INFO+2), a colon, the message, and the
// attributes as slog's text handler writes them (key=value, values quoted
// when needed, groups as dotted keys). There is no timestamp. A message
// with control characters is quoted.
//
// Records below the level in effect are dropped: warning, unless
// EnableVerbosity was called and --verbosity (or <EnvPrefix>_VERBOSITY,
// or the "verbosity" config key) chooses another. Run sets the level
// before the command's handlers run. The logger is safe for concurrent
// use, and the same logger is returned every time.
func (a *App) Logger() *slog.Logger {
	if a == nil {
		return stderrLogger()
	}
	return a.logs().logger
}

// LogLevel returns the level in effect for the app's logger (see Logger
// and EnableVerbosity), which Run updates. Use it to send the app's log
// records elsewhere at the same level:
//
//	h := slog.NewJSONHandler(f, &slog.HandlerOptions{Level: app.LogLevel()})
func (a *App) LogLevel() slog.Leveler {
	if a == nil {
		return slog.LevelWarn
	}
	return levelReader{&a.logs().level}
}

// levelReader exposes a LevelVar without its Set method.
type levelReader struct{ v *slog.LevelVar }

func (r levelReader) Level() slog.Level { return r.v.Level() }

// Logger returns the logger of the running App (see App.Logger). For a
// nil Context, or one without an App, it returns a logger that writes to
// os.Stderr at the warning level.
func (ctx *Context) Logger() *slog.Logger {
	if ctx == nil || ctx.App == nil {
		return stderrLogger()
	}
	return ctx.App.Logger()
}

// stderrLogger is the logger of a Context without an App.
var stderrLogger = sync.OnceValue(func() *slog.Logger {
	return slog.New(newLogHandler(func() io.Writer { return os.Stderr }, slog.LevelWarn))
})

// logHandler writes records as "LEVEL: message key=value ...". The
// attributes are formatted by a slog.TextHandler into a buffer, so
// WithAttrs and WithGroup behave exactly as they do for the text handler.
type logHandler struct {
	out   func() io.Writer
	level slog.Leveler
	state *logHandlerState // shared by the handlers derived with WithAttrs and WithGroup
	text  slog.Handler     // writes the attributes of a record into state.buf
}

// logHandlerState is what a logHandler and the handlers derived from it
// share. mu serializes records, so each one is a single Write.
type logHandlerState struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	builtins int // the built-in attributes (level, msg) still to drop from the record being written
}

func newLogHandler(out func() io.Writer, level slog.Leveler) *logHandler {
	st := &logHandlerState{}
	text := slog.NewTextHandler(&st.buf, &slog.HandlerOptions{
		Level: slog.Level(math.MinInt32), // Enabled is decided by logHandler
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// The text handler passes the built-in level and msg
			// attributes first (the time is zero and so left out); they
			// are written before the attributes instead. An attribute of
			// the record that happens to be named "msg" is kept.
			if len(groups) == 0 && st.builtins > 0 && (a.Key == slog.LevelKey || a.Key == slog.MessageKey) {
				st.builtins--
				return slog.Attr{}
			}
			return a
		},
	})
	return &logHandler{out: out, level: level, state: st, text: text}
}

func (h *logHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *logHandler) Handle(ctx context.Context, r slog.Record) error {
	st := h.state
	st.mu.Lock()
	defer st.mu.Unlock()
	st.buf.Reset()
	st.builtins = 2
	r.Time = time.Time{}
	err := h.text.Handle(ctx, r)
	st.builtins = 0
	if err != nil {
		return err
	}
	attrs := bytes.TrimSuffix(st.buf.Bytes(), []byte("\n"))
	line := make([]byte, 0, 16+len(r.Message)+len(attrs))
	line = append(line, levelName(r.Level)...)
	line = append(line, ':')
	if r.Message != "" {
		line = append(line, ' ')
		line = append(line, logMessage(r.Message)...)
	}
	if len(attrs) > 0 {
		line = append(line, ' ')
		line = append(line, attrs...)
	}
	line = append(line, '\n')
	_, err = h.out().Write(line)
	return err
}

func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	// The text handler formats the attributes now, calling ReplaceAttr,
	// which reads state.builtins.
	h.state.mu.Lock()
	text := h.text.WithAttrs(attrs)
	h.state.mu.Unlock()
	return &logHandler{out: h.out, level: h.level, state: h.state, text: text}
}

func (h *logHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &logHandler{out: h.out, level: h.level, state: h.state, text: h.text.WithGroup(name)}
}

// levelName names a level as gcloud does: DEBUG, INFO, WARNING, ERROR
// and CRITICAL; other levels as slog names them.
func levelName(l slog.Level) string {
	switch l {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARNING"
	case slog.LevelError:
		return "ERROR"
	case LevelCritical:
		return "CRITICAL"
	}
	return l.String()
}

// logMessage returns msg as it is, or quoted when it holds control
// characters or invalid UTF-8, so a message cannot move the cursor or
// forge a second log line.
func logMessage(msg string) string {
	for _, r := range msg {
		if r < 0x20 || (r >= 0x7f && r < 0xa0) || r == utf8.RuneError {
			return strconv.Quote(msg)
		}
	}
	return msg
}
