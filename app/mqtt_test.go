package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
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

func TestSplitFlightPayloadRecords(t *testing.T) {
	t.Run("single object payload", func(t *testing.T) {
		payload := []byte(`{"CMD":"DEP","CALLSIGN":"ABC123"}`)
		records, err := splitFlightPayloadRecords(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
	})

	t.Run("array payload", func(t *testing.T) {
		payload := []byte(`[{"CMD":"FPL"},{"CMD":"FPL"}]`)
		records, err := splitFlightPayloadRecords(payload)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		_, err := splitFlightPayloadRecords([]byte(`  `))
		if err == nil {
			t.Fatal("expected error for empty payload")
		}
	})
}

func TestMockupFLMOTypeSeparation(t *testing.T) {
	testCases := []struct {
		name       string
		fileName   string
		wantFlight bool
		wantPlan   bool
	}{
		{name: "arrival is non-FPL", fileName: "Arrival.json", wantFlight: true, wantPlan: false},
		{name: "cancel is non-FPL", fileName: "Cancel.json", wantFlight: true, wantPlan: false},
		{name: "change is non-FPL", fileName: "Change.json", wantFlight: true, wantPlan: false},
		{name: "delay is non-FPL", fileName: "Delay.json", wantFlight: true, wantPlan: false},
		{name: "departure is non-FPL", fileName: "Departure.json", wantFlight: true, wantPlan: false},
		{name: "flight plan is FPL", fileName: "Flight_Plan.json", wantFlight: false, wantPlan: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := readMockupFile(t, tc.fileName)
			records, err := splitFlightPayloadRecords(payload)
			if err != nil {
				t.Fatalf("unexpected split error: %v", err)
			}
			if len(records) == 0 {
				t.Fatalf("expected at least one record for %s", tc.fileName)
			}

			for _, record := range records {
				command, err := extractFlightCommand(record)
				if err != nil {
					t.Fatalf("unexpected command extraction error: %v", err)
				}

				if got := isNonFlightPlanCommand(command); got != tc.wantFlight {
					t.Fatalf("unexpected non-FPL classification for cmd=%s in %s: got=%t want=%t", command, tc.fileName, got, tc.wantFlight)
				}
				if got := isFlightPlanCommand(command); got != tc.wantPlan {
					t.Fatalf("unexpected FPL classification for cmd=%s in %s: got=%t want=%t", command, tc.fileName, got, tc.wantPlan)
				}
			}
		})
	}
}

func TestMockupTopicPayloads(t *testing.T) {
	t.Run("surveillance payload matches AS62 model", func(t *testing.T) {
		payload := readMockupFile(t, "Surveillance.json")
		var surv model.AODSSurveillance
		if err := json.Unmarshal(payload, &surv); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if surv.CallSign == "" {
			t.Fatal("expected surveillance callsign")
		}
	})

	t.Run("idep payload matches IDEP model", func(t *testing.T) {
		payload := readMockupFile(t, "iDep.json")
		var idep model.IDEP
		if err := json.Unmarshal(payload, &idep); err != nil {
			t.Fatalf("unexpected unmarshal error: %v", err)
		}
		if idep.AircraftID == "" {
			t.Fatal("expected idep aircraft id")
		}
	})
}

func readMockupFile(t *testing.T, fileName string) []byte {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file location")
	}

	projectRoot := filepath.Dir(filepath.Dir(currentFile))
	mockupPath := filepath.Join(projectRoot, "mockup", fileName)
	payload, err := os.ReadFile(mockupPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", mockupPath, err)
	}

	return payload
}
