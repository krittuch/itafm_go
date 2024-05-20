package repository

import (
	"database/sql"
	"fmt"

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
