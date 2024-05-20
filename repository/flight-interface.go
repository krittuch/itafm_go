package repository

import (
	"aerothai/itafm/model"
)

type FlightInterface interface {
	InsertFlight(*model.PostFlight)
	UpdateFlight(*model.PatchFlight) (*model.Flight, error)
}
