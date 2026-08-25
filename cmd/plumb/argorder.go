package main

import (
	"errors"
	"flag"
	"strings"
)

// parseFlags reorders args and parses them. The flag package writes
// its own message and the usage block to the set's output before it
// returns the error, so this wraps the error in a coded one: dispatch
// prints nothing for a coded error, and the message appears once
// instead of twice.
//
// The code is 2, the value the exit-code contract gives a usage
// error. This widens D-10, which gave a flag-parse error 1 and kept 2
// for a usage error dispatch raised itself. A pipeline reads 2 as
// "the command was called wrong" and 3 as "coverage fell", and the
// caller who types an unknown flag made the first kind of mistake.
//
// A help request keeps flag.ErrHelp, because dispatch answers that
// with exit code 0.
func parseFlags(fs *flag.FlagSet, args []string) error {
	err := fs.Parse(reorderArgs(fs, args))
	if err == nil || errors.Is(err, flag.ErrHelp) {
		return err
	}
	return newExitError(2, err.Error())
}

// boolFlag is the interface the flag package's own boolean flags
// implement. reorderArgs uses it to tell a flag that takes a value
// from one that does not.
type boolFlag interface {
	IsBoolFlag() bool
}

// reorderArgs moves every flag token, and the value it consumes,
// ahead of every positional argument. Go's flag.Parse stops reading
// flags at the first non-flag argument, so on its own it cannot
// parse "plumb check <profile> --min-statements N" or "plumb report
// <profile> --out file.html" — the forms the help text and the
// README document (D-23). Every command parses through this
// function, because a command that skips it drops a flag that comes
// after a positional argument and gives no warning.
//
// Everything after a bare "--" stays positional and is never scanned
// for flags, which matches the terminator flag.Parse itself honors.
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			continue // the value is attached; nothing more to consume
		}
		if name == "h" || name == "help" {
			continue // the built-in help flag takes no value
		}
		fl := fs.Lookup(name)
		if fl == nil {
			// An unknown flag consumes nothing. Taking the next token
			// could swallow a "--" terminator or a positional argument
			// before flag.Parse ever reports the flag as unknown.
			continue
		}
		if bf, ok := fl.Value.(boolFlag); ok && bf.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}

// flagsGiven returns the set of flag names the caller actually typed.
// A flag left at its default is absent from the map, so a command can
// tell "not given" from "given the zero value" — the technique D-33
// needs for the threshold guards and D-40 needs to know that
// --diff-base alone means diff mode. A sentinel default would break
// the moment a caller typed the sentinel; fs.Visit does not have that
// failure mode. Call it after parseFlags: fs.Visit reports nothing
// before the parse runs.
func flagsGiven(fs *flag.FlagSet) map[string]bool {
	given := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		given[f.Name] = true
	})
	return given
}
