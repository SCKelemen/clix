package plugin

import (
	"context"
	"os"
	"testing"

	"github.com/SCKelemen/clix/v2"
	"github.com/SCKelemen/clix/v2/ext/autocomplete"
	"github.com/SCKelemen/clix/v2/ext/format"
)

// appEnv makes the test binary act as the "probe" app, so a test can run
// it as a real process on a terminal.
const appEnv = "CLIX_PLUGIN_TEST_APP"

func TestMain(m *testing.M) {
	if os.Getenv(appEnv) == "1" {
		newProbeApp().Main(context.Background())
	}
	os.Exit(m.Run())
}

// newProbeApp returns the app the tests drive: plugins on, a "deploy"
// command, and the format and autocomplete extensions.
func newProbeApp() *clix.App {
	app := clix.NewApp("probe")
	app.Root.AddCommand(clix.NewCommand("deploy", clix.WithCommandShort("Deploy"), clix.WithCommandRun(func(*clix.Context) error { return nil })))
	app.AddExtension(Extension{})
	app.AddExtension(format.Extension{})
	app.AddExtension(autocomplete.Extension{})
	return app
}
