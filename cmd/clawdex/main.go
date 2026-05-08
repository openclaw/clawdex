package main

import (
	"fmt"
	"io"
	"os"

	"github.com/openclaw/clawdex/internal/cli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if err := cli.Execute(args, stdout, stderr); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return cli.ExitCode(err)
	}
	return 0
}
