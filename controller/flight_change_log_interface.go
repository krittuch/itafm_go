package controller

import (
	"aerothai/itafm/model"
)

type FlightChangeLogControllerInterface interface {
	FindAll() ([]model.FlightChangeLog, error)
	FindByFlightID(flightID uint) (model.FlightChangeLog, error)
	Insert(post model.PostFlightChangeLog) error
}
