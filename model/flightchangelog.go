package model

import "gopkg.in/guregu/null.v4"

type FlightChangeLog struct {
	ID        uint      `json:"id"`
	FlightID  uint      `json:"flight_id"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	CreatedAt null.Time `json:"created_at"`
	UpdatedAt null.Time `json:"updated_at"`
}

type PostFlightChangeLog struct {
	FlightID uint   `json:"flight_id"`
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}
