package model

// name
// iata
// icao
// call_sign
// country

type Airline struct {
	Name     string `json:"name"`
	IATA     string `json:"iata"`
	ICAO     string `json:"icao"`
	CallSign string `json:"call_sign"`
	Country  string `json:"country"`
}
