// cmd/plumb/main.go
package main

import (
	"fmt"
	"os"
)

// Set via -ldflags at release time. Default is "dev" so unreleased
// builds are obvious.
var version = "dev"

const usage = `plumb — better code coverage for Go

Usage:
  plumb <command> [flags] [args]

Commands:
  run      Run tests with coverage across packages

Run "plumb <command> -h" for command-specific help.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "report":
		err = reportCmd(args)
	case "-h", "--help", "help":
		fmt.Print(usage)
	case "version", "-v", "--version":
		fmt.Printf("plumb %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "plumb: %v\n", err)
		os.Exit(1)
	}
}
