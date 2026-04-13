package main

import (
	"os"
	"strings"

	"snapvector/cli"
	"snapvector/gui"
)

func main() {
	if isCLIInvocation(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
	}

	gui.Run()
}

func isCLIInvocation(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			return true
		}
	}

	return false
}
