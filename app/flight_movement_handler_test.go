package app

import "testing"

func TestParseFPLDOFFromItem18(t *testing.T) {
	item18 := "PBN/A1B1 DOF/260227 REG/B1319 EET/VLVT0104 ZPKM0114 RMK/TEST"
	got, ok := parseFPLDOFFromItem18(item18)
	if !ok {
		t.Fatal("expected DOF to be parsed")
	}
	if got != "260227" {
		t.Fatalf("unexpected DOF: got=%q expected=%q", got, "260227")
	}
}

func TestParseCHGRegisterFromItem18LikePayload(t *testing.T) {
	item18 := "PBN/A1B1 DOF/260227 REG/B1319 EET/VLVT0104 ZPKM0114 RMK/TEST"
	got, ok := parseCHGRegisterFromItem(item18)
	if !ok {
		t.Fatal("expected REG to be parsed")
	}
	if got != "B1319" {
		t.Fatalf("unexpected REG: got=%q expected=%q", got, "B1319")
	}
}

func TestBuildEstimateFromItem18EET(t *testing.T) {
	item18 := "PBN/A1B1C1D1L1O1S2 SUR/260 DOF/260227 REG/B1319 EET/VLVT0104 ZPKM0114 SEL/GPAF"
	got, ok := buildCHGArrivalEstimateFromEET(item18, "260227", "0800")
	if !ok {
		t.Fatal("expected EET estimate to be parsed")
	}
	if got != "2026-02-27 09:14:00+00" {
		t.Fatalf("unexpected estimate: got=%q expected=%q", got, "2026-02-27 09:14:00+00")
	}
}

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
