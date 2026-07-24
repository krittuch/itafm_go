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
	if *patchFlight.Bay != "" {
		flightController.UpdateBay(patchFlight.FlightNumber, data.EOBT, *patchFlight.Bay)
		updated = true
	}

	if !strings.Contains(data.TOBT, "0001-01-01") {
		flightController.UpdateTOBT(patchFlight.FlightNumber, data.TOBT)
		updated = true
	}

	return updated
}
