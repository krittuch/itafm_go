package controller

import (
	"database/sql"
	"log"

	"aerothai/itafm/model"
	"aerothai/itafm/repository"
)

type FlightControllerInterface interface {
	UpdateFlight(*model.PatchFlight) error
}

type FlightController struct {
	DB *sql.DB
}

func NewFlightController(db *sql.DB) *FlightController {
	return &FlightController{
		DB: db,
	}
}

func (f *FlightController) UpdateFlight(flight *model.PatchFlight) {

	repo := repository.NewFlightRepository(f.DB)
	err := repo.UpdateFlight(flight)

	if err != nil {
		log.Println(err)
	}
}
