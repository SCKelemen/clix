# Testing your CLI

A clix app runs in a test like any other function: build it, point its streams at buffers, and call `Run` with the arguments to test. The tests of [`examples/basic`](../examples/basic/cmd/demo/app_test.go), [`examples/gh`](../examples/gh/cmd/gh/app_test.go) and [`examples/gcloud`](../examples/gcloud/cmd/gcloud/app_test.go) do this.

## Running a command

In tests, call `app.Run(ctx, args)` with explicit arguments: `Run(ctx, nil)` reads `os.Args`, which under `go test` holds the test binary's own flags. `App.Main` exits the process, so tests never call it.

```go
import (
        "bytes"
        "testing"
)

func TestGreet(t *testing.T) {
        app := clix.NewApp("myapp") // or the constructor your main uses
        greet := clix.NewCommand("greet")
        var name string
        greet.Flags.StringVar(clix.StringVarOptions{
                FlagOptions: clix.FlagOptions{Name: "name", Required: true},
                Value:       &name,
        })
        greet.Run = func(ctx *clix.Context) error {
                _, err := fmt.Fprintf(ctx.App.Out, "Hello %s!\n", name)
                return err
        }
        app.Root.AddCommand(greet)

        var out bytes.Buffer
        app.Out = &out
        if err := app.Run(context.Background(), []string{"greet", "--name", "Alice"}); err != nil {
                t.Fatal(err)
        }
        if got := out.String(); got != "Hello Alice!\n" {
                t.Errorf("got %q", got)
        }
}
```

Flags also read the environment and the config file, so a developer's own `MYAPP_*` variables or `~/.config/myapp/config.yaml` can change what a test sees. Point `App.ConfigPath` at a file in `t.TempDir()`, or set `XDG_CONFIG_HOME` (or `HOME`, as the examples do) with `t.Setenv`, and set the variables a test depends on with `t.Setenv` too.

## Checking definitions in a test

`clix` tolerates definition mistakes at run time: a duplicate flag name keeps the first definition, a short name that is already taken is left off the later flag, a child name that collides with another child's alias is never reached. `App.Validate` reports all of them, along with defaults that do not parse, flags that shadow an ancestor's flag of the same name, and flag groups that name unknown flags. Call it in a test:

```go
import "testing"

func TestCLIDefinition(t *testing.T) {
        app := clix.NewApp("myapp") // or the constructor your main uses
        if err := app.Validate(); err != nil {
                t.Fatal(err)
        }
}
```

## Prompts

Readers that are not file descriptors, such as `strings.Reader`, are not terminals, so with the default `clix.InteractiveAuto` a missing required flag is a `*clix.MissingFlagsError` in a test. Tests that answer prompts from a string or a fake `Prompter` should set `clix.InteractiveAlways`. The default prompter is created by `NewApp`, so pass test streams as options rather than assigning `app.In` afterwards. Each prompt reads exactly one line, so one reader can answer several prompts in turn:

```go
import "bytes"

var prompts bytes.Buffer
app := clix.NewApp("myapp",
        clix.WithAppIn(strings.NewReader("Alice\n")),
        clix.WithAppErr(&prompts), // prompts go to App.Err
        clix.WithAppInteractive(clix.InteractiveAlways),
)
// ... add the greet command from the example above ...
if err := app.Run(context.Background(), []string{"greet"}); err != nil {
        return err // "Alice" answered the prompt for --name
}
```

A `Prompter` you set yourself, for example a fake that returns fixed answers, is used as it is, by `Run` and by the prompt extension alike (see [Non-interactive use](prompting-and-terminals.md#non-interactive-use)).

## Errors and exit codes

`Run` returns the error the process would exit with, so a test can check both the error and the status `clix.ExitCode` maps it to:

```go
app.Interactive = clix.InteractiveNever // never prompt, whatever In is
err := app.Run(context.Background(), []string{"greet"})
var missing *clix.MissingFlagsError
if errors.As(err, &missing) {
        fmt.Println(missing.Flags) // [name]: the long names, without dashes
}
fmt.Println(clix.ExitCode(err)) // 2, a usage error
```

`clix.ExitCode` returns 0 for success, 2 for usage errors, missing flags (`*clix.MissingFlagsError`) and broken flag groups (`*clix.FlagGroupError`), 130 for cancellation (`clix.ErrCancelled`, or `context.Canceled`), a plugin's own status for a `*clix.PluginExitError`, and 1 otherwise. `clix.UsageErrorf` makes an error that maps to 2, for usage problems a command finds while running.

## Running more than once

`Run` can be called repeatedly on one `App`, as tests and REPLs do. Each run first undoes what the previous run did: a flag that run set, from any source, goes back with its bound variable to the value it had when that run started, so `rm --force` followed by `rm` does not keep `--force`. A value your program assigned to a bound variable between two runs is kept instead. `Run` can tell an assignment from what the previous run left only by its value, so assigning exactly that value (`region = "eu"` after a run with `--region eu`) is undone like the rest; pass such a value on the command line, or make it the flag's `Default`.

Built-in values are put back exactly. A custom `Value` (see `FlagSet.Var`) is put back by its `Reset` method, which `Run` calls whenever the previous run called `Set` on it, whatever its `String` returns, so a secret that prints as `<redacted>` is reset too. `Run` never passes `String`'s text back to `Set`, so a custom value that should not start from its zero state needs a `Default`, which every run applies again. A custom `Value` without `Reset` keeps what a run set, and `Validate` reports that.

A handler may call `Run` itself, for an alias command or an interactive shell. The nested run starts from the outer run's values, with no flag set yet, and when it returns it puts back everything it changed, so the handler continues with its own values, `Changed` and `Source`, and nothing leaks into later runs. A custom value that the nested run set is reset rather than restored.

`Run` is not safe for concurrent use on one `App`; build one `App` per goroutine.

## Named configurations

`App.EnableConfigurations`, `CreateConfiguration`, `ActivateConfiguration` and the other [named configuration](configuration.md#named-configurations) methods set up configurations without running the `config configurations` commands.
