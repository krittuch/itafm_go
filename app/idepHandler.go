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

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-stomp/stomp/v3"
)

// Mock data as a global variable for demonstration purposes

func onIDEPReceive(
	msg *stomp.Message,
	db *sql.DB,
	flightController *controller.FlightController,
	client mqtt.Client) {
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

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
	icaoCode := airlineCodeRegex.FindString(data.AircraftID)

	iata, success := ConvertToIATA(icaoCode)

	if !success {
		log.Println("Cannot change flight number")
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

	patchFlight.FlightNumber = fmt.Sprint(iata, " ", flightNumber)

	if *patchFlight.Bay != "" {
		flightController.UpdateBay(patchFlight.FlightNumber, data.EOBT, *patchFlight.Bay)
		sendToITAFM(client, "server/trigger/flight/" + patchFlight.FlightNumber, "")
	}

	if data.TOBT != "" {
		flightController.UpdateTOBT(patchFlight.FlightNumber, data.TOBT)
		// log.Println("Success update TOBT" + patchFlight.FlightNumber)
		sendToITAFM(client, "server/trigger/flight/" + patchFlight.FlightNumber, "")
	}

}
