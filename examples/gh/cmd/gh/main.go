package main

import (
	"context"
	"fmt"
	"os"

	"clix/examples/gh/internal/gh"
)

func main() {
	app := gh.NewApp()

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(app.Err, err)
		os.Exit(1)
	}
}
