package repository

import (
	"aerothai/itafm/model"
)

type FlightInterface interface {
	UpdateFlight(*model.PatchFlight) (*model.Flight, error)
}
