package controller

import (
	"database/sql"

	"aerothai/itafm/model"
	"aerothai/itafm/repository"
)

type FlightChangeLogController struct {
	DB *sql.DB
}

func NewFlightChangeLogController(db *sql.DB) FlightChangeLogControllerInterface {
	return &FlightChangeLogController{
		DB: db,
	}
}

func (f *FlightChangeLogController) FindAll() ([]model.FlightChangeLog, error) {
	flightChangeLogRepository := repository.NewFlightChangeLogRepository(f.DB)
	return flightChangeLogRepository.FindAll()
}

func (f *FlightChangeLogController) FindByFlightID(flightID uint) (model.FlightChangeLog, error) {
	flightChangeLogRepository := repository.NewFlightChangeLogRepository(f.DB)
	return flightChangeLogRepository.FindByFlightID(flightID)
}

func (f *FlightChangeLogController) Insert(post model.PostFlightChangeLog) error {
	flightChangeLogRepository := repository.NewFlightChangeLogRepository(f.DB)
	return flightChangeLogRepository.Insert(post)
}
