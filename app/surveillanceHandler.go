package app

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/go-stomp/stomp/v3"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

func onSurveillanceReceive(msg *stomp.Message,
	db *sql.DB,
	surveillanceController *controller.SurveillanceController) {

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
}
