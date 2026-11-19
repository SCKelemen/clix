package clix_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/config"
	"github.com/SCKelemen/clix/v2/ext/help"
)

func topicNames(app *clix.App) []string {
	var names []string
	for _, t := range app.HelpTopics() {
		names = append(names, t.Name)
	}
	return names
}

func TestHelpTopics(t *testing.T) {
	app, out := newTestApp(t, "myapp")
	if got := topicNames(app); !reflect.DeepEqual(got, []string{"environment", "exit-codes"}) {
		t.Fatalf("built-in topics = %q", got)
	}
	app.AddHelpTopic("formatting", "Formatting output", "Use --format json.\n")
	app.AddHelpTopic("environment", "My environment", "MYAPP_TOKEN holds the token.")
	if got := topicNames(app); !reflect.DeepEqual(got, []string{"environment", "exit-codes", "formatting"}) {
		t.Fatalf("topics = %q", got)
	}
	if topic, ok := app.LookupHelpTopic("ENVIRONMENT"); !ok || topic.Short != "My environment" || topic.Body != "MYAPP_TOKEN holds the token." {
		t.Fatalf("the app's environment topic does not replace the built-in one: %+v", topic)
	}
	// Adding again replaces; an empty body removes, built-in ones too.
	app.AddHelpTopic("formatting", "Output formats", "Use --format.")
	app.AddHelpTopic("exit-codes", "", "")
	if got := topicNames(app); !reflect.DeepEqual(got, []string{"environment", "formatting"}) {
		t.Fatalf("topics = %q", got)
	}
	if topic, _ := app.LookupHelpTopic("formatting"); topic.Short != "Output formats" {
		t.Fatalf("formatting = %+v", topic)
	}

	// Without a help command, root help does not list topics no one can
	// read.
	out.Reset()
	if err := app.Run(bg, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "ADDITIONAL HELP TOPICS") {
		t.Fatalf("topics listed without a help command:\n%s", out.String())
	}

	app.AddExtension(help.Extension{})
	out.Reset()
	if err := app.Run(bg, []string{"--help"}); err != nil {
		t.Fatal(err)
	}
	want := "ADDITIONAL HELP TOPICS\n  environment          My environment\n  formatting           Output formats\n\n"
	if !strings.HasSuffix(out.String(), want) {
		t.Fatalf("root help:\n%s\nwant it to end with:\n%s", out.String(), want)
	}
	// Only root help lists them.
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandRun(noop)))
	out.Reset()
	if err := app.Run(bg, []string{"deploy", "--help"}); err != nil || strings.Contains(out.String(), "ADDITIONAL HELP TOPICS") {
		t.Fatalf("deploy --help: %v\n%s", err, out.String())
	}

	out.Reset()
	if err := app.Run(bg, []string{"help", "formatting"}); err != nil || out.String() != "Output formats\n\nUse --format.\n" {
		t.Fatalf("help formatting: %v\n%q", err, out.String())
	}
	out.Reset()
	if err := app.Run(bg, []string{"help", "Formatting"}); err != nil || !strings.HasPrefix(out.String(), "Output formats") {
		t.Fatalf("help Formatting: %v\n%q", err, out.String())
	}
	for _, args := range [][]string{{"help", "exit-codes"}, {"help", "formatting", "extra"}} {
		if err := app.Run(bg, args); err == nil {
			t.Errorf("%q succeeded", args)
		}
	}
	if err := app.Run(bg, []string{"help", "formating"}); err == nil || !strings.Contains(err.Error(), `did you mean "formatting"?`) {
		t.Fatalf("help formating: %v", err)
	}
}

// A command beats a topic of the same name, and Validate reports it.
func TestHelpTopicsShadowedByCommands(t *testing.T) {
	app, out := newTestApp(t, "myapp")
	app.AddExtension(help.Extension{})
	ran := false
	app.Root.AddCommand(clix.NewCommand("environment", clix.WithCommandShort("Manage environments"), clix.WithCommandRun(mark(&ran))))
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandAliases("ship"), clix.WithCommandRun(noop)))
	app.AddHelpTopic("ship", "Shipping", "How to ship.")
	app.AddHelpTopic("two words", "Bad", "Cannot be typed.")
	if got := topicNames(app); !reflect.DeepEqual(got, []string{"exit-codes", "two words"}) {
		t.Fatalf("topics = %q", got)
	}
	out.Reset()
	if err := app.Run(bg, []string{"help", "environment"}); err != nil || !strings.HasPrefix(out.String(), "Manage environments") {
		t.Fatalf("help environment shows the command: %v\n%s", err, out.String())
	}
	assertValidate(t, app,
		`myapp: help topic "ship" is shadowed by the command "deploy"; "help ship" shows the command`,
		`myapp: help topic name "two words" cannot be typed as one word`,
	)
}

// The app's topics win over an extension's, whatever the order, and an
// extension's win over the built-in ones.
func TestHelpTopicsOwnership(t *testing.T) {
	for _, order := range registrationOrders {
		t.Run(order, func(t *testing.T) {
			app, _ := newTestApp(t, "myapp")
			ext := extensionFunc(func(a *clix.App) error {
				a.AddHelpTopic("formatting", "From the extension", "ext")
				a.AddHelpTopic("exit-codes", "Extension exit codes", "ext")
				return nil
			})
			withOrder(t, app, order, []clix.Extension{ext, help.Extension{}}, func() {
				app.AddHelpTopic("formatting", "From the app", "app")
			})
			_ = app.ApplyExtensions()
			if topic, _ := app.LookupHelpTopic("formatting"); topic.Short != "From the app" {
				t.Errorf("formatting = %q, want the app's", topic.Short)
			}
			if topic, _ := app.LookupHelpTopic("exit-codes"); topic.Short != "Extension exit codes" {
				t.Errorf("exit-codes = %q, want the extension's", topic.Short)
			}
			assertValidate(t, app)
		})
	}
}

// The built-in topics describe what clix does for this app.
func TestBuiltinHelpTopics(t *testing.T) {
	app, _ := newTestApp(t, "my-app")
	app.Flags().StringVar(clix.StringVarOptions{FlagOptions: clix.FlagOptions{Name: "project", Usage: "Project to use", EnvVars: []string{"GCP_PROJECT"}}})
	app.Flags().BoolVar(clix.BoolVarOptions{FlagOptions: clix.FlagOptions{Name: "force", CommandLineOnly: true}})
	app.AddExtension(config.Extension{Configurations: true})
	_ = app.ApplyExtensions()
	env, _ := app.LookupHelpTopic("environment")
	for _, want := range []string{
		"my-app reads these environment variables.",
		"  MY_APP_<FLAG>           The value of a flag: --dry-run reads MY_APP_DRY_RUN,",
		"  MY_APP_PROJECT          --project: Project to use\n",
		"  GCP_PROJECT             --project: Project to use\n",
		"  MY_APP_CONFIGURATION    The named configuration to use, instead of the active",
		"  MY_APP_DISABLE_PROMPTS  Set to 1, true, yes or on to never prompt",
		"  XDG_CONFIG_HOME ",
		"  NO_COLOR ",
		"  CLICOLOR ",
		"  CLICOLOR_FORCE ",
		"  TERM ",
	} {
		if !strings.Contains(env.Body, want) {
			t.Errorf("environment lacks %q:\n%s", want, env.Body)
		}
	}
	if strings.Contains(env.Body, "MY_APP_FORCE") || strings.Contains(env.Body, "MY_APP_HELP") {
		t.Errorf("environment lists a variable no flag reads:\n%s", env.Body)
	}
	for _, line := range strings.Split(env.Body, "\n") {
		if len(line) > 80 {
			t.Errorf("line longer than 80 columns: %q", line)
		}
	}

	// It follows the app: AutoEnv off, a fixed config path.
	off := false
	app.AutoEnv = &off
	app.ConfigPath = "/etc/my-app.yaml"
	env, _ = app.LookupHelpTopic("environment")
	if strings.Contains(env.Body, "<FLAG>") || strings.Contains(env.Body, "MY_APP_PROJECT") || strings.Contains(env.Body, "XDG_CONFIG_HOME") || !strings.Contains(env.Body, "GCP_PROJECT") {
		t.Errorf("environment with AutoEnv off and ConfigPath:\n%s", env.Body)
	}

	codes, _ := app.LookupHelpTopic("exit-codes")
	for _, want := range []string{"  0    Success.", "  1    An error.", "  2    A usage error", "  130  Cancelled", "  143  Terminated by SIGTERM."} {
		if !strings.Contains(codes.Body, want) {
			t.Errorf("exit-codes lacks %q:\n%s", want, codes.Body)
		}
	}
}

func TestRenderTopic(t *testing.T) {
	app, _ := newTestApp(t, "myapp")
	var b bytes.Buffer
	if err := (clix.HelpRenderer{App: app}).RenderTopic(&b, clix.HelpTopic{Name: "x", Body: "body\n\n"}); err != nil || b.String() != "x\n\nbody\n" {
		t.Fatalf("RenderTopic = %q, %v", b.String(), err)
	}
}
