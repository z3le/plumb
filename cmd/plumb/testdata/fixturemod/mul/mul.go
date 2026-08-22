// Package mul holds a single small function used to prove that a
// package with no test file of its own is credited with the coverage
// its callers' tests produce.
package mul

// Double returns n multiplied by two.
func Double(n int) int {
	return n * 2
}
