package app

import (
	"encoding/json"

	"aerothai/itafm/model"
)

func onSurveillanceReceive(body []byte, batcher *surveillanceBatcher) bool {

	survData := model.AODSSurveillance{}
	err := json.Unmarshal(body, &survData)

	if err != nil {
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

	if batcher == nil {
		return false
	}

	if !batcher.add(&survData) {
		return false
	}
	return true
}
