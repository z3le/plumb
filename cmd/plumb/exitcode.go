// cmd/plumb/exitcode.go
package main

// exitError carries a process exit code. A command that returns an
// *exitError has already written its own report to the stderr writer
// it received, so dispatch returns the code and prints nothing more
// (D-30, D-31) — the same rule dispatch already applies to
// flag.ErrHelp.
type exitError struct {
	code int
	msg  string
}

// newExitError builds a coded error. msg is the value Error()
// returns; it exists for Go's error interface and for a test
// assertion, not for a second print to the user.
func newExitError(code int, msg string) *exitError {
	return &exitError{code: code, msg: msg}
}

func (e *exitError) Error() string {
	return e.msg
}

// ExitCode reports the process exit code this error carries. dispatch
// reads it through errors.As, so a wrapped *exitError still resolves.
func (e *exitError) ExitCode() int {
	return e.code
}
