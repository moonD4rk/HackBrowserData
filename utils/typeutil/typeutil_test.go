package typeutil

import (
	"testing"
)

func TestReverse(t *testing.T) {
	t.Parallel()

	reverseTestCases := [][]any{
		{1, 2, 3, 4, 5},
		{"1", "2", "3", "4", "5"},
		{"1", 2, "3", "4", 5},
	}

	for _, ts := range reverseTestCases {
		h := Reverse(ts)
		for i := 0; i < len(ts); i++ {
			if h[len(h)-i-1] != ts[i] {
				t.Errorf("reverse failed %v != %v", h, ts)
			}
		}
	}
}
