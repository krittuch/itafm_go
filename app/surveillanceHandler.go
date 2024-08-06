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

	success := false
	survData.CallSign, success = ConvertToIATA(survData.CallSign)

	if !success {
		log.Println("Cannot change flight number")
		return
	}

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
