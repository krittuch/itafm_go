package app

import (
	"database/sql"
	"encoding/json"

	"github.com/go-stomp/stomp/v3"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

func onSurveillanceReceive(msg *stomp.Message,
	db *sql.DB,
	surveillanceController *controller.SurveillanceController) {

	aodsMsg := model.PostSurveillance{}
	err := json.Unmarshal(msg.Body, &aodsMsg)

	if err != nil {
		return
	}

	if aodsMsg.Departure != "VTBS" || aodsMsg.Destination != "VTBS" {
		return
	}

	surveillanceController.InsertOrUpdateSurveillance(&aodsMsg)
}
