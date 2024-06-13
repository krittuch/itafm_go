package controller

import (
	"database/sql"
	"log"

	"aerothai/itafm/model"
	"aerothai/itafm/repository"
)

type FlightControllerInterface interface {
	InsertFlight(*model.PostFlight)
	UpdateFlight(*model.PatchFlight) error
	UpdateDepartureFlight(string, string, string)
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

func (f *FlightController) InsertFlight(flight *model.PostFlight) {
	repo := repository.NewFlightRepository(f.DB)
	err := repo.InsertFlight(flight)

	if err != nil {
		log.Println(err)
	}
}

func (f *FlightController) UpdateDepartureFlight(flightNumber string, date string, datetime string) {
	repo := repository.NewFlightRepository(f.DB)
	datetime = 
	err := repo.UpdateFlight()
}
