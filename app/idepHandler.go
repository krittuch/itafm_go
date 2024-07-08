package app

import (
	"aerothai/itafm/controller"
	"aerothai/itafm/model"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/go-stomp/stomp/v3"
)

// Mock data as a global variable for demonstration purposes

func onIDEPReceive(
	msg *stomp.Message,
	db *sql.DB,
	flightController *controller.FlightController) {
	// Simulate receiving message

	data := model.IDEP{}
	err := json.Unmarshal(msg.Body, &data)
	if err != nil {
		log.Println("Error unmarshalling IDEP data:", err)
		return
	}

	patchFlight := model.PatchFlight{
		Bay: &data.DepartureParkingStand,
	}

	airlineController := controller.NewAirlineController(db)

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
	icaoCode := airlineCodeRegex.FindString(data.AircraftID)

	airline, errAirline := airlineController.GetAirline(icaoCode)

	if errAirline != nil {
		return
	}

	// Get numberic number without 0 prefix from data.AircraftID
	numberRegex := regexp.MustCompile(`\d+`)
	matchString := numberRegex.FindString(data.AircraftID)
	if len(matchString) <= 0 {
		log.Println("Cannot find number in ", data.AircraftID)
		return
	}
	flightNumber := strings.TrimLeft(matchString, "0")

	patchFlight.FlightNumber = fmt.Sprint(airline.IATA, " ", flightNumber)

	flightController.UpdateBay(patchFlight.FlightNumber, data.EOBT, *patchFlight.Bay)

}
