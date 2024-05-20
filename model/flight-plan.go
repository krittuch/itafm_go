package model

// Get this from AODS
type FlightPlan struct {
	CMD         string `json:"CMD"`
	CALLSIGN    string `json:"CALLSIGN"`
	FRULE       string `json:"FRULE"`
	NUM         string `json:"NUM"`
	ACTYPE      string `json:"ACTYPE"`
	WTURB       string `json:"WTURB"`
	COMNAV      string `json:"COMNAV"`
	DEPARTURE   string `json:"DEPARTURE"`
	ETD         string `json:"ETD"`
	SPEED       string `json:"SPEED"`
	FLEVEL      string `json:"FLEVEL"`
	ROUTE       string `json:"ROUTE"`
	DESTINATION string `json:"DESTINATION"`
	EET         string `json:"EET"`
	ALTN        string `json:"ALTN"`
	ALTN2       string `json:"ALTN2"`
	DOF         string `json:"DOF"`
	ITEM18      string `json:"ITEM18"`
}
