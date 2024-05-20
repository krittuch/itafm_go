package model

type AODSFlightMovement struct {
	CMD         string `json:"CMD"`
	CALLSIGN    string `json:"CALLSIGN"`
	SQUAWK      string `json:"SQUAWK"`
	DEPARTURE   string `json:"DEPARTURE"`
	TIME1       string `json:"TIME1"`
	DESTINATION string `json:"DESTINATION"`
	ARRIVAL     string `json:"ARRIVAL"`
	TIME2       string `json:"TIME2"`
	DOF         string `json:"DOF"`
	ITEM18      string `json:"ITEM18"`
}

type Command struct {
	CMD string `json:CMD`
}
