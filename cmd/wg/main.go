package main

import (
	"context"
	"os"

	"wg/internal/cli"
	"wg/internal/git"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}

	code := cli.Run(context.Background(), os.Args[1:], cli.Options{
		Cwd:       cwd,
		Stdin:     os.Stdin,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Environ:   os.Environ(),
		GitRunner: git.ExecRunner{Binary: "git"},
	})
	os.Exit(code)
}
