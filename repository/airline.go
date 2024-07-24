package repository

import (
	"database/sql"
	"log"
	"aerothai/itafm/model"
)

type AirlineRepository struct {
	DB *sql.DB
}

func NewAirlineRepository(db *sql.DB) *AirlineRepository {
	return &AirlineRepository{
		DB: db,
	}
}

func (a *AirlineRepository) GetAirline(icaoCode string) (model.Airline, error) {

	var (
		name     string
		iata     string
		icao     string
		callsign string
		country  string
	)

	err := a.DB.QueryRow(`
	SELECT name, iata, icao, call_sign, country 
	FROM flight_airlinecode 
	WHERE icao = $1`,
		icaoCode).Scan(
		&name,
		&iata,
		&icao,
		&callsign,
		&country)

	if err != nil {
		log.Println(icaoCode)
		return model.Airline{}, err
	}

	airline := model.Airline{
		Name:     name,
		IATA:     iata,
		ICAO:     icao,
		CallSign: callsign,
		Country:  country,
	}

	return airline, nil
}
