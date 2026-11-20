package clix

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/SCKelemen/clix/v2/internal/answer"
)

// quietFlagName is the long name of the flag EnableQuiet registers.
const quietFlagName = "quiet"

// ErrQuiet is wrapped, together with ErrPromptsDisabled, in the error
// NewPromptConfig returns when --quiet is in effect (see App.EnableQuiet).
// A Prompter that gets it should not prompt but return QuietAnswer(cfg),
// as the built-in prompters do; one that returns the error instead fails
// safe. Test for it with errors.Is.
var ErrQuiet = errors.New("--quiet is set")

// QuietOptions configure EnableQuiet.
type QuietOptions struct {
	// Short is the short name of --quiet, such as "q" (gcloud's -q). It is
	// empty by default, because the format extension gives -q to --jq, as
	// gh does; see EnableQuiet for how the two are resolved.
	Short string
}

// quietValue is the Value of the --quiet flag that EnableQuiet registers.
// Its type tells that flag apart from an app flag of the same name, which
// wins and is then not quiet mode.
type quietValue struct{ on bool }

func (v *quietValue) Set(s string) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	v.on = b
	return nil
}

func (v *quietValue) SetBool(b bool) error {
	v.on = b
	return nil
}

func (v *quietValue) String() string { return strconv.FormatBool(v.on) }

// IsBoolFlag makes --quiet a switch.
func (v *quietValue) IsBoolFlag() bool { return true }

// Reset lets one App run more than once (see FlagSet.Var).
func (v *quietValue) Reset() { v.on = false }

// EnableQuiet registers the global --quiet switch, gcloud's: it turns off
// every interactive prompt for one command and accepts defaults instead.
//
//   - Run never prompts for missing required flags: IsInteractive reports
//     false, whatever App.Interactive says, so a missing required flag is
//     a *MissingFlagsError.
//   - A prompt from command code is answered without reading input or
//     drawing anything: the built-in prompters return QuietAnswer, the
//     prompt's Default (a confirmation answers as an empty reply would,
//     Yes unless its Default is no), and fail with an error wrapping
//     ErrPromptsDisabled when there is no default to accept. Custom
//     prompters built on NewPromptConfig get an error wrapping ErrQuiet
//     and should return QuietAnswer too.
//   - Context.Progress prints nothing, as with ProgressQuiet.
//
// --quiet is Ambient and read from the command line only (see
// FlagOptions.CommandLineOnly): because it accepts the default answer of
// a confirmation, it confirms destructive operations, so no environment
// variable or config key can turn it on for every run. That is where clix
// differs from gcloud, whose --quiet is CLOUDSDK_CORE_DISABLE_PROMPTS=1:
// in clix, <EnvPrefix>_DISABLE_PROMPTS keeps failing closed (prompts
// return ErrPromptsDisabled and nothing is answered for the user), and
// --quiet, when both apply, wins and accepts the defaults.
//
// The flag has no short name unless opts.Short sets one. gcloud's -q is
// --quiet, but the format extension (ext/format) gives -q to --jq, as gh
// does. When the app calls EnableQuiet(QuietOptions{Short: "q"}) itself,
// the flag is the app's, and the app's flags win over extensions'
// whatever the order (see App.ApplyExtensions): -q is --quiet and --jq
// keeps only its long name, without a conflict for App.Validate to
// report. Called from inside an extension's Extend, --quiet belongs to
// that extension, and between extensions the one added first wins.
//
// A --quiet flag the app defines itself is resolved like any other name
// conflict: it wins over EnableQuiet called from an extension, and
// between two of the app's own flags the one registered first is kept
// (App.Validate reports the other). When the app's own --quiet is the
// one in effect, it is an ordinary flag: Quiet reports false. Calling
// EnableQuiet again does nothing.
func (a *App) EnableQuiet(opts QuietOptions) {
	if a.quiet {
		return
	}
	a.quiet = true
	a.Flags().Var(VarOptions{
		FlagOptions: FlagOptions{
			Name:            quietFlagName,
			Short:           opts.Short,
			Usage:           "Disable all interactive prompts; use defaults where input is needed, or fail",
			Ambient:         true,
			CommandLineOnly: true,
		},
		Value: &quietValue{},
	})
}

// Quiet reports whether --quiet (see EnableQuiet) is in effect: it was
// given on the command line of the Run in progress, or of the last one.
// It is false without EnableQuiet, and when an app flag named --quiet
// replaced the one EnableQuiet registered.
func (a *App) Quiet() bool {
	if a == nil {
		return false
	}
	f := a.rootFlagSet().lookup(quietFlagName)
	if f == nil {
		return false
	}
	v, ours := f.Value.(*quietValue)
	return ours && v.on
}

// QuietAnswer returns the answer a prompt gets under --quiet (see
// App.EnableQuiet), without reading input or writing anything:
//
//   - A confirmation answers as an empty reply does: "y", unless Default
//     is "n", "no", "false", "f", "0" or "off" (see PromptRequest.Confirm).
//   - A select prompt returns the Value of the option Default names (by
//     value, then by label).
//   - A multi-select prompt returns the values of the options Default
//     names, joined with commas in option order (an option without a
//     Value contributes its Label), as the prompt extension does.
//   - A text or secret prompt returns Default.
//
// The answer must pass cfg.Validate. Any prompt but a confirmation that
// has no Default is an error that wraps ErrPromptsDisabled and ErrQuiet:
// there is no default to accept.
func QuietAnswer(cfg *PromptConfig) (string, error) {
	if cfg == nil {
		return "", errors.New("clix: QuietAnswer needs a prompt configuration")
	}
	noDefault := func() (string, error) {
		return "", fmt.Errorf("cannot prompt for %q: %w (%w) and it has no default to accept", cfg.Label, ErrPromptsDisabled, ErrQuiet)
	}
	var value string
	switch {
	case cfg.Confirm:
		value = answer.Normalize(answer.DefaultYes(cfg.Default))
	case (cfg.Options != nil || cfg.MultiSelect) && len(cfg.Options) == 0:
		return "", fmt.Errorf("cannot prompt for %q: %w", cfg.Label, ErrNoOptions)
	case cfg.MultiSelect:
		if strings.TrimSpace(cfg.Default) == "" {
			return noDefault()
		}
		idx, ok := ResolveMultiSelectDefault(cfg.Options, cfg.Default)
		if !ok {
			return "", fmt.Errorf("cannot prompt for %q: %w: %q", cfg.Label, ErrInvalidDefault, cfg.Default)
		}
		if len(idx) == 0 {
			return noDefault()
		}
		selected := make(map[int]bool, len(idx))
		for _, i := range idx {
			selected[i] = true
		}
		var values []string
		for i, o := range cfg.Options {
			if !selected[i] {
				continue
			}
			if o.Value != "" {
				values = append(values, o.Value)
			} else {
				values = append(values, o.Label)
			}
		}
		value = strings.Join(values, ",")
	case len(cfg.Options) > 0:
		if cfg.Default == "" {
			return noDefault()
		}
		values, labels := optionNames(cfg.Options)
		i, ok := answer.SingleDefault(values, labels, cfg.Default)
		if !ok {
			return "", fmt.Errorf("cannot prompt for %q: %w: %q", cfg.Label, ErrInvalidDefault, cfg.Default)
		}
		value = cfg.Options[i].Value
	default:
		if cfg.Default == "" {
			return noDefault()
		}
		value = cfg.Default
	}
	if cfg.Validate != nil {
		if err := cfg.Validate(value); err != nil {
			if cfg.Secret {
				// Never show a secret, not even inside a validation message.
				err = errors.New(answer.RedactSecret(err.Error(), value))
			}
			return "", fmt.Errorf("cannot accept the default for %q under --quiet: %w", cfg.Label, err)
		}
	}
	return value, nil
}
