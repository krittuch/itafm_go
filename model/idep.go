package model

type IDEP struct {
	AircraftID            string  `json:"AircraftID"`
	Departure             string  `json:"Departure"`
	Destination           string  `json:"Destination"`
	EOBT                  string  `json:"EOBT"`
	DepartureRunway       string  `json:"DepartureRunway"`
	ArrivalRunway         string  `json:"ArrivalRunway"`
	DepartureParkingStand string  `json:"DepartureParkingStand"`
	ArrivalParkingStand   string  `json:"ArrivalParkingStand"`
	EXOT                  float32 `json:"EXOT"`
	TOBT                  string  `json:"TOBT"`
	TSAT                  string  `json:"TSAT"`
	AIBT                  string  `json:"AIBT"`
	AOBT                  string  `json:"AOBT"`
	URNO                  string  `json:"URNO"`
}
