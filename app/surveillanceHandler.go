package app

import (
	"encoding/json"

	"aerothai/itafm/model"
)

func onSurveillanceReceive(body []byte, batcher *surveillanceBatcher) surveillanceReceiveResult {

	survData := model.AODSSurveillance{}
	err := json.Unmarshal(body, &survData)

	if err != nil {
		return surveillanceReceiveResult{Outcome: surveillanceOutcomeDecodeError}
	}

	result := surveillanceReceiveResult{RawCallSign: survData.CallSign, Data: &survData}

	if survData.Departure != "VTBS" && survData.Destination != "VTBS" {
		result.Outcome = surveillanceOutcomeNotVTBS
		return result
	}

	success := false
	survData.CallSign, success = ConvertToIATA(survData.CallSign)
	result.IATACallSign = survData.CallSign

	if !success {
		result.Outcome = surveillanceOutcomeIATAFailed
		return result
	}

	if batcher == nil {
		result.Outcome = surveillanceOutcomeRejected
		return result
	}

	result.Outcome = batcher.queue(&survData)
	return result
}
