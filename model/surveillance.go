package model

import "encoding/json"

type Surveillance struct {
	Id              uint    `json:"id"`
	CallSign        string  `json:"callsign"`
	Departure       string  `json:"departure"`
	Dest            string  `json:"destination"`
	AircraftType    string  `json:"actype"`
	WakeTurbulance  string  `json:"wturbulance"`
	Lat             float64 `json:"latitude"`
	Lon             float64 `json:"longitude"`
	Altitude        float64 `json:"altitude"`
	GroundSpeed     float64 `json:"gspeed"`
	Heading         float64 `json:"heading"`
	AircraftAddress string  `json:"acaddress"`
	SIC             int     `json:"sic"`
	SAC             int     `json:"sac"`
	SSRCode         string  `json:"ssrcode"`
	DateTime        string  `json:"datetime"`
	TrackNumber     int     `json:"trackno"`
	VX              float64 `json:"vx"`
	VY              float64 `json:"vy"`
	CDM             string  `json:"cdm"`
}

type PostSurveillance struct {
	CallSign        string  `json:"callsign"`
	Departure       string  `json:"departure"`
	Destination     string  `json:"destination"`
	AircraftType    string  `json:"actype"`
	WakeTurbulance  string  `json:"wturbulance"`
	Lat             float64 `json:"latitude"`
	Lon             float64 `json:"longitude"`
	Altitude        float64 `json:"altitude"`
	GroundSpeed     float64 `json:"gspeed"`
	Heading         float64 `json:"heading"`
	AircraftAddress string  `json:"acaddress"`
	SIC             int     `json:"sic"`
	SAC             int     `json:"sac"`
	SSRCode         string  `json:"ssrcode"`
	DateTime        string  `json:"datetime"`
	TrackNumber     int     `json:"trackno"`
	VX              float64 `json:"vx"`
	VY              float64 `json:"vy"`
	CDM             string  `json:"cdm"`
}

type AODSSurveillance struct {
	CallSign        string  `json:"CallSign"`
	Departure       string  `json:"Dep"`
	Destination     string  `json:"Dest"`
	AircraftType    string  `json:"AircraftType"`
	WakeTurbulance  string  `json:"WakeTurbulance"`
	Lat             float64 `json:"Lat"`
	Lon             float64 `json:"Lon"`
	Altitude        float64 `json:"Altitude"`
	GroundSpeed     float64 `json:"GroundSpeed"`
	Heading         float64 `json:"Heading"`
	AircraftAddress string  `json:"AircraftAddress"`
	SIC             int     `json:"SIC"`
	SAC             int     `json:"SAC"`
	SSRCode         string  `json:"SSRCode"`
	DateTime        string  `json:"datetime"`
	TrackNumber     int     `json:"trackNumber"`
	VX              float64 `json:"vx"`
	VY              float64 `json:"vy"`
	CDM             string  `json:"cdm"`
}

func (a *AODSSurveillance) UnmarshalJSON(data []byte) error {
	type alias AODSSurveillance
	aux := struct {
		*alias
		LegacyDateTime string `json:"dt"`
	}{
		alias: (*alias)(a),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if a.DateTime == "" {
		a.DateTime = aux.LegacyDateTime
	}

	return nil
}
