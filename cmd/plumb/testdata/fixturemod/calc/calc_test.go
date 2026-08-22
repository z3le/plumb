package calc

import "testing"

func TestAddDoubled(t *testing.T) {
	if got := AddDoubled(2, 3); got != 8 {
		t.Fatalf("AddDoubled(2, 3) = %d, want 8", got)
	}
}
