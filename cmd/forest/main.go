package main

import (
	"os"

	"github.com/oluwatimilehin/git-forest/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
