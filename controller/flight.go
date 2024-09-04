package controller

import (
	"database/sql"
	"log"

	"aerothai/itafm/model"
	"aerothai/itafm/repository"
)

type FlightControllerInterface interface {
	GetFlight(string, string) model.Flight
	InsertFlight(*model.PostFlight)
	UpdateFlight(*model.PatchFlight) error
	UpdateBay(string, string, string)
	UpdateDepartureFlight(string, string, string)
	UpdateCallsign( string,  string)
}

type FlightController struct {
	DB *sql.DB
}

func NewFlightController(db *sql.DB) *FlightController {
	return &FlightController{
		DB: db,
	}
}

func (f *FlightController) GetFlight(flightNumber string, std string) (model.Flight, error) {
	repo := repository.NewFlightRepository(f.DB)
	return repo.GetFlight(flightNumber, std)
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
		log.Println("Error on Insert Flight : ", err)
	}
}

func (f *FlightController) UpdateDepartureFlight(flightNumber string, date string, datetime string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateDepartureFlight(flightNumber, date, datetime)

	if err != nil {
		log.Println(err)
	}
}

func (f *FlightController) UpdateBay(flightNumber string, std string, bay string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateBay(flightNumber, std, bay)

	if err != nil {
		log.Println(err)
	}
}


func (f *FlightController) UpdateTOBT(flightNumber string, tobt string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateTOBTFlight(flightNumber, tobt)

	if err != nil {
		log.Println(err)
	}
}

func (f *FlightController) UpdateCallsign(callsign string, tobt string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateCallsign(callsign, tobt)

	if err != nil {
		log.Println(err)
	}
}