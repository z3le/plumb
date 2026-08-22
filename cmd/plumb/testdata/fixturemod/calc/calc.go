// Package calc holds small arithmetic functions for the run fixture.
package calc

import "example.com/fixturemod/mul"

// Add returns a plus b.
func Add(a, b int) int {
	return a + b
}

// AddDoubled returns a plus b doubled.
func AddDoubled(a, b int) int {
	return Add(a, mul.Double(b))
}
