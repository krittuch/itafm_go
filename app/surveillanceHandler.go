package app

import (
	"database/sql"
	"encoding/json"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

func onSurveillanceReceive(body []byte,
	db *sql.DB,
	surveillanceController *controller.SurveillanceController,
	client mqtt.Client) {

	survData := model.AODSSurveillance{}
	err := json.Unmarshal(body, &survData)

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
		return
	}

	// surveillanceController.InsertOrUpdateSurveillance(&survData)

	//Convert survData to string
	survDataString, errMashal := json.Marshal(survData)
	if errMashal != nil {
		log.Println(errMashal)
		return

	}

	// Also Send to itafm
	sendToITAFM(client, *itafmSurvTopicName, string(survDataString))
}
