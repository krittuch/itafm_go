package repository

import (
	"aerothai/itafm/model"
)

type FlightInterface interface {
	GetFlight(string, string) (model.Flight, error)
	InsertFlight(*model.PostFlight)
	UpdateFlight(*model.PatchFlight) (*model.Flight, error)
	UpdateDepartureFlight(string, string, string) error
	UpdateArrivalFlight(string, string, string) error
	UpdateCanceledFlight(string, string) error
	UpdateDelayedFlight(string, string) error
	UpdateBay(string, string, string) error
	UpdateTOBTFlight(string, string) error
	UpdateRegister(string, string, string) error
}
