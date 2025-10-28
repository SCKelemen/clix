package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	app := newApp()

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(app.Err, err)
		os.Exit(1)
	}
}
