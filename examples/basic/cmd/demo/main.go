package main

import (
	"context"
	"fmt"
	"os"

	"clix/examples/basic/internal/demo"
)

func main() {
	app := demo.NewApp()

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(app.Err, err)
		os.Exit(1)
	}
}
