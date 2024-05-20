package controller

import (
	"database/sql"

	"aerothai/itafm/model"
	"aerothai/itafm/repository"
)

type AirlineInterface interface {
	GetAirline(string) (model.Airline, error)
}

type AirlineController struct {
	DB *sql.DB
}

func NewAirlineController(db *sql.DB) *AirlineController {
	return &AirlineController{
		DB: db,
	}
}

func (a *AirlineController) GetAirline(icaoCode string) (model.Airline, error) {
	repo := repository.NewAirlineRepository(a.DB)
	return repo.GetAirline(icaoCode)
}
