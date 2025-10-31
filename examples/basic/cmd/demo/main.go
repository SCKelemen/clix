package main

import (
	"context"
	"fmt"
	"os"

	demoapp "clix/examples/basic/internal/app"
)

func main() {
	application := demoapp.New()

	if err := application.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(application.Err, err)
		os.Exit(1)
	}
}
