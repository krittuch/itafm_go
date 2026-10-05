package app

import (
	"aerothai/itafm/controller"
	"aerothai/itafm/model"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Mock data as a global variable for demonstration purposes

func onIDEPReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) bool {
	// Simulate receiving message

	data := model.IDEP{}
	err := json.Unmarshal(body, &data)
	if err != nil {
		return false
	}

	patchFlight := model.PatchFlight{
		Bay: &data.DepartureParkingStand,
	}

	icaoCode := airlineCodeRegex.FindString(data.AircraftID)

	iata, success := ConvertToIATA(icaoCode)

	if !success {
		return false
	}

	// Get numberic number without 0 prefix from data.AircraftID
	matchString := numberRegex.FindString(data.AircraftID)
	if len(matchString) <= 0 {
		return false
	}
	flightNumber := strings.TrimLeft(matchString, "0")

	patchFlight.FlightNumber = fmt.Sprint(iata, " ", flightNumber)

	updated := false
	if bay, eobt, ok := idepBayUpdate(data); ok {
		flightController.UpdateBay(patchFlight.FlightNumber, eobt, bay)
		updated = true
	}

	if !strings.Contains(data.TOBT, "0001-01-01") {
		flightController.UpdateTOBT(patchFlight.FlightNumber, data.TOBT)
		updated = true
	}

	return updated
}

// idepBayUpdate returns the bay and EOBT to write for an IDEP message.
// DepartureParkingStand is a stand at the departure airport, so only a known
// VTBS stand on a VTBS departure is used; other airports' stands used to
// overwrite the bay of the same flight number arriving at VTBS.
func idepBayUpdate(data model.IDEP) (string, string, bool) {
	bay := strings.TrimSpace(data.DepartureParkingStand)
	eobt := strings.TrimSpace(data.EOBT)
	if strings.TrimSpace(data.Departure) != "VTBS" || eobt == "" || !isVTBSStand(bay) {
		return "", "", false
	}
	return bay, eobt, true
}
