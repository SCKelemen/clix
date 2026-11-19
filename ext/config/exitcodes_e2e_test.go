package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
)

// Refusals caused by what was asked for are usage errors (exit status 2),
// like a bad flag; failures to read or write files stay 1.
func TestConfigRefusalsAreUsageErrors(t *testing.T) {
	c := newCfgApp(t)
	c.app.Flags().IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "retries", Usage: "Retries"}})
	c.app.Flags().IntVar(clix.IntVarOptions{FlagOptions: clix.FlagOptions{Name: "pin", Usage: "PIN", Sensitive: true}})
	c.run(t, "config", "configurations", "create", "work") // created and activated
	c.run(t, "config", "configurations", "create", "old", "--no-activate")
	c.run(t, "config", "configurations", "create", "spare", "--no-activate")
	c.run(t, "config", "set", "token", "s3cret")

	for _, tc := range []struct {
		name string
		args []string
		msg  string
	}{
		// configurations delete
		{"delete the active configuration", []string{"config", "configurations", "delete", "-y", "work"}, `cannot delete the active configuration "work"`},
		{"delete default", []string{"config", "configurations", "delete", "-y", "default"}, `cannot delete the "default" configuration`},
		{"delete the configuration in use", []string{"--configuration", "old", "config", "configurations", "delete", "-y", "old"}, `cannot delete configuration "old": this command uses it`},
		{"delete a missing configuration", []string{"config", "configurations", "delete", "-y", "nope"}, `configuration "nope" does not exist`},
		{"delete an invalid name", []string{"config", "configurations", "delete", "-y", "Bad"}, `invalid configuration name "Bad"`},
		// configurations rename
		{"rename default", []string{"config", "configurations", "rename", "default", "x"}, `cannot rename the "default" configuration`},
		{"rename the active configuration", []string{"config", "configurations", "rename", "work", "x"}, `cannot rename the active configuration "work"`},
		{"rename the configuration in use", []string{"--configuration", "old", "config", "configurations", "rename", "old", "x"}, `cannot rename configuration "old": this command uses it`},
		{"rename onto an existing name", []string{"config", "configurations", "rename", "old", "spare"}, `configuration "spare" already exists`},
		{"rename a missing configuration", []string{"config", "configurations", "rename", "nope", "x"}, `configuration "nope" does not exist`},
		{"rename to an invalid name", []string{"config", "configurations", "rename", "old", "Bad"}, `invalid configuration name "Bad"`},
		// configurations activate, create, describe
		{"activate a missing configuration", []string{"config", "configurations", "activate", "nope"}, `configuration "nope" does not exist`},
		{"activate an invalid name", []string{"config", "configurations", "activate", "../x"}, `invalid configuration name "../x"`},
		{"create an existing configuration", []string{"config", "configurations", "create", "spare"}, `configuration "spare" already exists`},
		{"create default", []string{"config", "configurations", "create", "default"}, `configuration "default" already exists`},
		{"create an invalid name", []string{"config", "configurations", "create", "Bad"}, `invalid configuration name "Bad"`},
		{"describe a missing configuration", []string{"config", "configurations", "describe", "nope"}, `configuration "nope" does not exist`},
		{"describe an invalid name", []string{"config", "configurations", "describe", "Bad"}, `invalid configuration name "Bad"`},
		// get, set, unset
		{"get an empty key", []string{"config", "get", " "}, "key path cannot be empty"},
		{"get a key with a space", []string{"config", "get", "a b"}, `key path "a b" must not contain spaces`},
		{"get a secret without --reveal", []string{"config", "get", "token"}, `config key "token" holds a secret; add --reveal to print it`},
		{"set a key with a space", []string{"config", "set", "a b", "x"}, `key path "a b" must not contain spaces`},
		{"set an empty value", []string{"config", "set", "project", " "}, "value cannot be empty"},
		{"set a value the flag rejects", []string{"config", "set", "retries", "many"}, `invalid value "many" for config key "retries", which sets --retries`},
		{"unset an empty key", []string{"config", "unset", " "}, "key path cannot be empty"},
		{"unset a key with a space", []string{"config", "unset", "a b"}, `key path "a b" must not contain spaces`},
	} {
		_, err := c.try(tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.msg) || clix.ExitCode(err) != 2 {
			t.Errorf("%s (%q): %v (exit %d), want %q (exit 2)", tc.name, tc.args, err, clix.ExitCode(err), tc.msg)
		}
	}

	// A missing configuration is still ErrConfigurationNotFound.
	for _, args := range [][]string{
		{"config", "configurations", "delete", "-y", "nope"},
		{"config", "configurations", "activate", "nope"},
		{"config", "configurations", "describe", "nope"},
	} {
		if _, err := c.try(args...); !errors.Is(err, clix.ErrConfigurationNotFound) {
			t.Errorf("%q: %v does not wrap ErrConfigurationNotFound", args, err)
		}
	}

	// A key that is not set is not a mistake in the call: like
	// "git config --get", config get reports it with exit status 1.
	if _, err := c.try("config", "get", "no.such.key"); err == nil || clix.ExitCode(err) != 1 {
		t.Errorf("config get no.such.key: %v (exit %d), want exit 1", err, clix.ExitCode(err))
	}
	// A value rejected for a secret key is a usage error too, and still
	// does not show the value.
	if _, err := c.try("config", "set", "pin", "s3cret"); err == nil || strings.Contains(err.Error(), "s3cret") || clix.ExitCode(err) != 2 {
		t.Errorf("config set pin s3cret: %v (exit %d), want exit 2 without the value", err, clix.ExitCode(err))
	}
}

// The App methods report the same refusals as usage errors, so a program
// that calls them with its user's input gets exit status 2 from ExitCode.
func TestConfigurationMethodsRefuseWithUsageErrors(t *testing.T) {
	c := newCfgApp(t)
	c.run(t, "config", "configurations", "create", "work")
	c.run(t, "config", "configurations", "create", "old", "--no-activate")
	for name, err := range map[string]error{
		"CreateConfiguration(Bad)":         c.app.CreateConfiguration("Bad"),
		"CreateConfiguration(old)":         c.app.CreateConfiguration("old"),
		"ActivateConfiguration(nope)":      c.app.ActivateConfiguration("nope"),
		"DeleteConfiguration(work)":        c.app.DeleteConfiguration("work"),
		"DeleteConfiguration(default)":     c.app.DeleteConfiguration(clix.DefaultConfiguration),
		"RenameConfiguration(old, work)":   c.app.RenameConfiguration("old", "work"),
		"RenameConfiguration(old, ../x)":   c.app.RenameConfiguration("old", "../x"),
		"RenameConfiguration(nope, other)": c.app.RenameConfiguration("nope", "other"),
	} {
		if err == nil || clix.ExitCode(err) != 2 {
			t.Errorf("%s = %v (exit %d), want a usage error", name, err, clix.ExitCode(err))
		}
	}
	if _, err := c.app.ConfigurationFile("Bad"); err == nil || clix.ExitCode(err) != 2 {
		t.Errorf("ConfigurationFile(Bad) = %v (exit %d), want a usage error", err, clix.ExitCode(err))
	}
}

// Failures of the file system are not usage errors.
func TestConfigFileFailuresExitOne(t *testing.T) {
	t.Run("create without a configurations directory", func(t *testing.T) {
		c := newCfgApp(t)
		// A file where the configurations directory should be.
		c.write(t, "configurations", "not a directory\n")
		if _, err := c.try("config", "configurations", "create", "work"); err == nil || clix.ExitCode(err) != 1 {
			t.Errorf("create: %v (exit %d), want exit 1", err, clix.ExitCode(err))
		}
	})
	t.Run("reset of a directory", func(t *testing.T) {
		c := newCfgApp(t)
		if err := os.MkdirAll(filepath.Join(c.dir, "config.yaml"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err := c.try("config", "reset")
		if err == nil || !strings.Contains(err.Error(), "not a regular file") || clix.ExitCode(err) != 1 {
			t.Errorf("reset: %v (exit %d), want exit 1", err, clix.ExitCode(err))
		}
	})
}
