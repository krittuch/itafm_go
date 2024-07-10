package app

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/go-stomp/stomp/v3"
	mqtt "github.com/eclipse/paho.mqtt.golang" 

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


	surveillanceController.InsertOrUpdateSurveillance(&survData)

	// Also Send to itafm
	sendToITAFM(client, *itafmSurvTopicName, string(msg.Body))
}
