// cmd/plumb/argorder.go
package main

import (
	"flag"
	"strings"
)

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
		if fl := fs.Lookup(name); fl != nil {
			if bf, ok := fl.Value.(boolFlag); ok && bf.IsBoolFlag() {
				continue
			}
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}
