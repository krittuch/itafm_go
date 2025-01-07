package repository

import (
	"aerothai/itafm/model"
)

type FlightChangeLogRepositoryInterface interface {
	FindAll() ([]model.FlightChangeLog, error)
	FindByFlightID(flightID uint) (model.FlightChangeLog, error)
	Insert(postFlightChangeLog model.PostFlightChangeLog) error
}
