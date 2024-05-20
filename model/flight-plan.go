package model

// Get this from AODS
type FlightPlan struct {
	Id             int    `json:"id"`
	Cmd            string `json:"command"`
	Callsign       string `json:"callsign"`
	FlightRule     string `json:"flight_rule"`
	Num            string `json:"number"`
	AircraftType   string `json:"aircraft_type"`
	WindTurbulance string `json:"wind_turbulance"`
	ComNav         string `json:"comnav"`
	Departure      string `json:"departure"`
	Etd            string `json:"estimate_time_departure"`
	Speed          string `json:"speed"`
	FlightLevel    string `json:"flight_level"`
	Route          string `json:"route"`
	Destination    string `json:"destination"`
	Eet            string `json:"estimated_elapsed_time"`
	Alternative    string `json:"alternative"`
	Alternative2   string `json:"alternative2"`
	DOF            string `json:"date_of_flight"`
	Item18         string `json:"item18"`
}
