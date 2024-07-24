package app

import (
	"database/sql"
	"encoding/json"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/go-stomp/stomp/v3"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

func onSurveillanceReceive(msg *stomp.Message,
	db *sql.DB,
	surveillanceController *controller.SurveillanceController,
	client mqtt.Client) {

	survData := model.AODSSurveillance{}
	err := json.Unmarshal(msg.Body, &survData)

	if err != nil {
		log.Println(err)
		return
	}

	if survData.Departure != "VTBS" && survData.Destination != "VTBS" {
		return
	}

	// Change to IATA code
	airlineController := controller.NewAirlineController(db)
	if len(survData.CallSign) < 3 {
		return
	}
	icaoCode := survData.CallSign[:3]
	airline, err := airlineController.GetAirline(icaoCode)

	if err != nil {
		log.Println(err)
		return
	}

	survData.CallSign = airline.IATA + survData.CallSign[3:]

	surveillanceController.InsertOrUpdateSurveillance(&survData)

	//Convert survData to string
	survDataString, errMashal := json.Marshal(survData)
	if errMashal != nil {
		log.Println(errMashal)
		return

	}

	// Also Send to itafm
	sendToITAFM(client, *itafmSurvTopicName, string(survDataString))
}
