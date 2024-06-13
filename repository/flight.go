package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"aerothai/itafm/model"
)

type FlightRepository struct {
	DB *sql.DB
}

func NewFlightRepository(db *sql.DB) *FlightRepository {
	return &FlightRepository{
		DB: db,
	}
}

func (f *FlightRepository) UpdateFlight(flight *model.PatchFlight) error {

	qString := ""

	if flight.ActualFlightTime != nil {
		qString += fmt.Sprintf(`actual_flight_time = '%s', `, flight.ActualFlightTime.Format("2006-01-02 15:04:05"))
	}

	if flight.EstimateFlightTime != nil {
		qString += fmt.Sprintf(`estimate_flight_time = '%s', `, flight.EstimateFlightTime.Format("2006-01-02 15:04:05"))
	}

	if flight.ScheduleFlightTime != nil {
		qString += fmt.Sprintf(`schedule_flight_time = '%s', `, flight.ScheduleFlightTime.Format("2006-01-02 15:04:05"))
	}

	if flight.Canceled != nil {
		qString += fmt.Sprintf(`canceled = %t, `, *flight.Canceled)
	}

	if flight.Bay != nil {
		qString += fmt.Sprintf(`bay = '%s', `, *flight.Bay)
	}

	if flight.Gate != nil {
		qString += fmt.Sprintf(`gate = '%s', `, *flight.Gate)
	}

	if qString != "" {
		qString = qString[:len(qString)-2]
	}

	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET $1 WHERE id = $2`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(qString, flight.ID)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) InsertFlight(flight *model.PostFlight) error {

	stmt, err := f.DB.Prepare(`INSERT INTO flight_flight 
	(aircraft, type, schedule_flight_time, flight_number, next_station, prev_station,
	 working, finished, canceled, created_at, updated_at
	) 
	VALUES ($1, $2, $3, $4, $5, $6, false, false, false, now(), now())`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(
		flight.AircraftType,
		flight.Type,
		flight.ScheduleFlightTime,
		flight.FlightNumber,
		flight.NextStation,
		flight.PrevStation)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateDepartureFlight(flightNumber string, date string, time string) error {

	aft = strings.Join(string[date[]], "")
	
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET actual_flight_time=$1 
	WHERE flight_number = $2 & schedule_flight_time >= $3`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(qString, flight.ID)

	if err2 != nil {
		return err2
	}

	return nil
}