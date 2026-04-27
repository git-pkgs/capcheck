package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/git-pkgs/capcheck/cmd/capcheck/app"
)

const exitError = 2

func main() {
	if err := app.New().Execute(); err != nil {
		var ee app.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.Code)
		}
		fmt.Fprintln(os.Stderr, "capcheck:", err)
		os.Exit(exitError)
	}
}
