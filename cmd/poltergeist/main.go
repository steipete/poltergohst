package main

import (
	"fmt"
	"os"

	"github.com/poltergeist/poltergeist/pkg/cli"
)

var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
