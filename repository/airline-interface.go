package repository

import (
	"aerothai/itafm/model"
)

type AirlineInterface interface {
	GetAirline(string) (model.Airline, error)
}
