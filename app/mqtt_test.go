package app

import (
	"reflect"
	"testing"

	"aerothai/itafm/model"
)

func TestSplitBrokers(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single broker",
			input:    "localhost:9092",
			expected: []string{"localhost:9092"},
		},
		{
			name:     "multiple brokers with spaces",
			input:    " localhost:9092, kafka-2:9092 ,kafka-3:9092 ",
			expected: []string{"localhost:9092", "kafka-2:9092", "kafka-3:9092"},
		},
		{
			name:     "ignores empty entries",
			input:    "localhost:9092,, ,kafka-2:9092,",
			expected: []string{"localhost:9092", "kafka-2:9092"},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitBrokers(tc.input)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("unexpected brokers: got=%v expected=%v", got, tc.expected)
			}
		})
	}
}

func TestConvertToIATA(t *testing.T) {
	originalAirlines := airlines
	t.Cleanup(func() {
		airlines = originalAirlines
	})

	airlines = []*model.CSVAirline{
		{ICAO: "THA", IATA: "TG"},
		{ICAO: "UAE", IATA: "EK"},
	}

	testCases := []struct {
		name          string
		input         string
		expectedValue string
		expectedOk    bool
	}{
		{
			name:          "convert callsign with suffix",
			input:         "THA616",
			expectedValue: "TG616",
			expectedOk:    true,
		},
		{
			name:          "convert just icao code",
			input:         "UAE",
			expectedValue: "EK",
			expectedOk:    true,
		},
		{
			name:          "too short returns false",
			input:         "TH",
			expectedValue: "TH",
			expectedOk:    false,
		},
		{
			name:          "unknown airline returns false",
			input:         "ABC123",
			expectedValue: "ABC123",
			expectedOk:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotValue, gotOk := ConvertToIATA(tc.input)
			if gotValue != tc.expectedValue || gotOk != tc.expectedOk {
				t.Fatalf(
					"unexpected conversion result: got=(%q,%t) expected=(%q,%t)",
					gotValue,
					gotOk,
					tc.expectedValue,
					tc.expectedOk,
				)
			}
		})
	}
}
