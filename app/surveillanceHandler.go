package app

import (
	"encoding/json"
	"log"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

func onSurveillanceReceive(body []byte, surveillanceController *controller.SurveillanceController) bool {

	survData := model.AODSSurveillance{}
	err := json.Unmarshal(body, &survData)

	if err != nil {
		log.Println(err)
		return false
	}

	if survData.Departure != "VTBS" && survData.Destination != "VTBS" {
		return false
	}

	success := false
	survData.CallSign, success = ConvertToIATA(survData.CallSign)

	if !success {
		return false
	}

	if !surveillanceController.InsertOrUpdateSurveillance(&survData) {
		log.Println("failed to insert/update surveillance record")
		return false
	}
	return true
}
