package main

import (
	"context"
	"fmt"
	"os"

	"clix/examples/gcloud/internal/gcloud"
)

func main() {
	app := gcloud.NewApp()

	if err := app.Run(context.Background(), nil); err != nil {
		fmt.Fprintln(app.Err, err)
		os.Exit(1)
	}
}
