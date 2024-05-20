package model

import (
	"time"
)

type Flight struct {
	ID                 int        `json:"id"`
	Aircraft           string     `json:"ac_register"`
	ActualFlightTime   *time.Time `json:"actual_flight_time"`
	AircraftType       string     `json:"aircraft"`
	Bay                string     `json:"bay"`
	Belt               string     `json:"belt"`
	DepartureDate      *time.Time `json:"departure_date"`
	EstimateFlightTime *time.Time `json:"estimate_flight_time"`
	FlightNumber       string     `json:"flight_number"`
	Gate               string     `json:"gate"`
	NextStation        string     `json:"next_station"`
	ScheduleFlightTime *time.Time `json:"schedule_flight_time"`
	Sequence           string     `json:"sequence"`
	Canceled           bool       `json:"canceled"`
	Finished           bool       `json:"finished"`
	Working            bool       `json:"working"`
	CreatedAt          *time.Time `json:"created_at"`
	UpdatedAt          *time.Time `json:"updated_at"`
	BookingPax         string     `json:"booking_pax"`
	PrevStation        string     `json:"prev_station"`
	Type               string     `json:"type"`
	RelateFlightID     *int       `json:"relate_flight_id,omitempty"`
}

type PatchFlight struct {
	ID                 int        `json:"id"`
	ActualFlightTime   *time.Time `json:"actual_flight_time"`
	EstimateFlightTime *time.Time `json:"estimate_flight_time"`
	ScheduleFlightTime *time.Time `json:"schedule_flight_time"`
	Canceled           *bool      `json:"canceled"`
	Bay                *string    `json:"bay"`
	Gate               *string    `json:"gate"`
}
