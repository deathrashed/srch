package main

import (
	"fmt"
	"os"

	"srch/internal/app"
	"srch/internal/cli"
)

var version = "dev"

func main() {
	environment, err := app.New()
	if environment == nil {
		fmt.Fprintln(os.Stderr, "srch:", err)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "srch: warning:", err)
	}
	root := cli.New(environment, version)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "srch:", err)
		os.Exit(1)
	}
}
