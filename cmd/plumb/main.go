// cmd/plumb/main.go
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// Set via -ldflags at release time. Default is "dev" so unreleased
// builds are obvious.
var version = "dev"

// command is one entry in the command registry. The registry is the
// single source for both dispatch and the --help text, so a command
// cannot exist without being listed and cannot be listed without
// existing.
type command struct {
	name    string
	summary string
	run     func(args []string, stdout, stderr io.Writer) error
}

// allCommands returns every command plumb supports, in a fixed order.
// The order is part of the contract.
//
// This is a function, not a package-level variable, because helpCmd
// calls usage(), and usage() reads the registry. A var initializer
// with that shape is an initialization cycle that Go refuses to
// compile. A function body is not part of package initialization, so
// every entry — help included — keeps its run field here as data.
func allCommands() []command {
	return []command{
		{name: "run", summary: "Run tests with coverage and render the report", run: runCmd},
		{name: "report", summary: "Render a coverage profile as an HTML report", run: reportCmd},
		{name: "check", summary: "Check coverage against a minimum threshold", run: checkCmd},
		{name: "version", summary: "Print the plumb version", run: versionCmd},
		{name: "help", summary: "Show this help text", run: helpCmd},
	}
}

// aliases maps a short or long flag form to a command name. Aliases are
// not registry entries, so they never appear in the help text.
var aliases = map[string]string{
	"-h":        "help",
	"--help":    "help",
	"-v":        "version",
	"--version": "version",
}

// lookup finds a command by exact byte equality — no case folding and
// no Unicode normalization.
func lookup(name string) (command, bool) {
	for _, c := range allCommands() {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// usage writes the top-level help text. The command lines are produced
// by ranging over commands, not written as a literal string, so the
// help text cannot drift from the dispatch again.
func usage(w io.Writer) {
	fmt.Fprint(w, "plumb — better code coverage for Go\n\n")
	fmt.Fprint(w, "Usage:\n")
	fmt.Fprint(w, "  plumb <command> [flags] [args]\n\n")
	fmt.Fprint(w, "Commands:\n")
	for _, c := range allCommands() {
		fmt.Fprintf(w, "  %-8s %s\n", c.name, c.summary)
	}
	fmt.Fprint(w, "\n")
	fmt.Fprint(w, "Run \"plumb <command> -h\" for command-specific help.\n")
}

// helpCmd prints the top-level usage text to stdout.
func helpCmd(args []string, stdout, stderr io.Writer) error {
	usage(stdout)
	return nil
}

// versionCmd prints the plumb version to stdout.
func versionCmd(args []string, stdout, stderr io.Writer) error {
	fmt.Fprintf(stdout, "plumb %s\n", version)
	return nil
}

// dispatch resolves the command name, runs it, and returns the process
// exit code. The exit-code table is fixed for the whole phase:
//
//	0 — the command succeeded, or the caller asked for help
//	1 — the command returned an ordinary error, which includes a
//	    profile that measures no coverable statement
//	2 — a usage error: no argument, an unknown command, an unknown
//	    flag, a flag value the command cannot read, a second file
//	    name, or a threshold outside 0 to 100
//	3 — a coded error for a coverage threshold that the profile did
//	    not meet
//
// A pipeline reads 2 as "the command was called wrong" and 3 as
// "coverage fell", so every wrong call answers with 2.
func dispatch(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	name := args[0]
	if aliased, ok := aliases[name]; ok {
		name = aliased
	}
	cmd, ok := lookup(name)
	if !ok {
		fmt.Fprintf(stderr, "unknown command: %s\n\n", name)
		usage(stderr)
		return 2
	}
	err := cmd.run(args[1:], stdout, stderr)
	if err == nil {
		return 0
	}
	// flag.ContinueOnError already wrote the command's own usage block
	// to its output while parsing. Writing anything more here would
	// print that block a second time, so a help request returns 0 and
	// writes nothing further.
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	// A coded error means the command already wrote its own report to
	// the writer it received (D-30, D-31), so dispatch only returns
	// the code and prints nothing more — the same rule the flag.ErrHelp
	// branch above follows.
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	fmt.Fprintf(stderr, "plumb: %v\n", err)
	return 1
}

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdout, os.Stderr))
}
