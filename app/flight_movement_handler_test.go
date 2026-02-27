package app

import "testing"

func TestParseCHGAircraftFromItem(t *testing.T) {
	testCases := []struct {
		name     string
		item     string
		expected string
		ok       bool
	}{
		{
			name:     "extracts aircraft from multiline item 9 segment",
			item:     "-9/A320/M\r\n-",
			expected: "A320",
			ok:       true,
		},
		{
			name:     "extracts and normalizes lowercase value",
			item:     "-9/a321/m REG/HS-TAA",
			expected: "A321",
			ok:       true,
		},
		{
			name: "ignores non item 9 payload",
			item: "-15/N0120A035 DCT",
			ok:   false,
		},
		{
			name: "does not match other item numbers",
			item: "-19/A320/M",
			ok:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseCHGAircraftFromItem(tc.item)
			if ok != tc.ok {
				t.Fatalf("unexpected parse status: got=%t expected=%t", ok, tc.ok)
			}
			if got != tc.expected {
				t.Fatalf("unexpected aircraft: got=%q expected=%q", got, tc.expected)
			}
		})
	}
}
