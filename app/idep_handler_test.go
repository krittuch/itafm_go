package app

import (
	"testing"

	"aerothai/itafm/model"
)

func TestIDEPBayUpdateAcceptsVTBSDepartureStand(t *testing.T) {
	bay, eobt, ok := idepBayUpdate(model.IDEP{
		Departure:             "VTBS",
		Destination:           "VTSS",
		EOBT:                  "2023-10-03T04:15:00",
		DepartureParkingStand: " G1 ",
	})
	if !ok || bay != "G1" || eobt != "2023-10-03T04:15:00" {
		t.Fatalf("got bay=%q eobt=%q ok=%t", bay, eobt, ok)
	}
}

func TestIDEPBayUpdateRejectsUnusableMessages(t *testing.T) {
	cases := map[string]model.IDEP{
		"other airport": {Departure: "VTSP", EOBT: "2026-10-04T03:20:00", DepartureParkingStand: "C3"},
		"unknown stand": {Departure: "VTBS", EOBT: "2026-10-04T03:20:00", DepartureParkingStand: "9"},
		"empty stand":   {Departure: "VTBS", EOBT: "2026-10-04T03:20:00", DepartureParkingStand: ""},
		"missing EOBT":  {Departure: "VTBS", DepartureParkingStand: "C3"},
	}
	for name, data := range cases {
		if _, _, ok := idepBayUpdate(data); ok {
			t.Errorf("%s: bay must not be written", name)
		}
	}
}

func TestIsVTBSStand(t *testing.T) {
	for _, bay := range []string{"C3", "110", "110R", "s127", " A2 "} {
		if !isVTBSStand(bay) {
			t.Errorf("%q should be a VTBS stand", bay)
		}
	}
	for _, bay := range []string{"", "3", "9", "C11", "115R", "D9"} {
		if isVTBSStand(bay) {
			t.Errorf("%q should not be a VTBS stand", bay)
		}
	}
}
