package controller

import (
	"database/sql"
	"log"

	"aerothai/itafm/model"
	"aerothai/itafm/repository"
)

type FlightControllerInterface interface {
	GetFlight(string, string) (model.Flight, error)
	GetFlightByTypeAndSchedule(string, string, string) (model.Flight, error)
	FindDepartureFlightByDestinationAndDate(string, string, string) (model.Flight, error)
	FindArrivalFlightByRouteAndDate(string, string, string, string) (model.Flight, error)
	InsertFlight(*model.PostFlight)
	UpdateFlight(*model.PatchFlight) error
	UpdateBay(string, string, string)
	UpdateDepartureFlight(string, string, string)
	UpdateArrivalFlight(string, string, string)
	UpdateCanceledFlight(string, string)
	UpdateDelayedFlight(string, string)
	UpdateEstimateFlightByDestination(string, string, string, string)
	UpdateArrivalEstimateFlightByRoute(string, string, string, string, string)
	UpdateRegister(string, string, string)
	UpdateAircraft(string, string, string)
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

func (f *FlightController) GetFlightByTypeAndSchedule(flightNumber string, flightType string, std string) (model.Flight, error) {
	repo := repository.NewFlightRepository(f.DB)
	return repo.GetFlightByTypeAndSchedule(flightNumber, flightType, std)
}

func (f *FlightController) FindDepartureFlightByDestinationAndDate(flightNumber string, destination string, date string) (model.Flight, error) {
	repo := repository.NewFlightRepository(f.DB)
	return repo.FindDepartureFlightByDestinationAndDate(flightNumber, destination, date)
}

func (f *FlightController) FindArrivalFlightByRouteAndDate(flightNumber string, departure string, destination string, date string) (model.Flight, error) {
	repo := repository.NewFlightRepository(f.DB)
	return repo.FindArrivalFlightByRouteAndDate(flightNumber, departure, destination, date)
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

func (f *FlightController) UpdateArrivalFlight(flightNumber string, date string, datetime string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateArrivalFlight(flightNumber, date, datetime)

	if err != nil {
		log.Println(err)
	}
}

func (f *FlightController) UpdateCanceledFlight(flightNumber string, std string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateCanceledFlight(flightNumber, std)

	if err != nil {
		log.Println(err)
	}
}

func (f *FlightController) UpdateDelayedFlight(flightNumber string, std string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateDelayedFlight(flightNumber, std)

	if err != nil {
		log.Println(err)
	}
}

func (f *FlightController) UpdateEstimateFlightByDestination(flightNumber string, destination string, date string, datetime string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateEstimateFlightByDestination(flightNumber, destination, date, datetime)

	if err != nil {
		log.Println(err)
	}
}

func (f *FlightController) UpdateArrivalEstimateFlightByRoute(flightNumber string, departure string, destination string, date string, datetime string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateArrivalEstimateFlightByRoute(flightNumber, departure, destination, date, datetime)

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

func (f *FlightController) UpdateRegister(callsign string, tobt string, std string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateRegister(callsign, tobt, std)

	if err != nil {
		log.Println(err)
	}
}

func (f *FlightController) UpdateAircraft(flightNumber string, aircraft string, std string) {
	repo := repository.NewFlightRepository(f.DB)

	err := repo.UpdateAircraft(flightNumber, aircraft, std)

	if err != nil {
		log.Println(err)
	}
}
