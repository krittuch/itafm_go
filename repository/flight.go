package repository

import (
	"database/sql"
	"fmt"
	"time"

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

func (f *FlightRepository) GetFlight(fn string, std string) (model.Flight, error) {
	var (
		id                 int
		flightNumber       string
		scheduleFlightTime *time.Time
		acRegister         string
		actualFlightTime   *time.Time
		estimateFlightTime *time.Time
		canceled           bool
		delayed            bool
	)

	err := f.DB.QueryRow(`
	SELECT id, flight_number, schedule_flight_time, COALESCE(ac_register, ''), actual_flight_time, estimate_flight_time, COALESCE(canceled, FALSE), COALESCE(delayed, FALSE)
	FROM flight_flight
	WHERE flight_number = $1 and schedule_flight_time = $2`,
		fn, std).Scan(
		&id,
		&flightNumber,
		&scheduleFlightTime,
		&acRegister,
		&actualFlightTime,
		&estimateFlightTime,
		&canceled,
		&delayed,
	)

	if err != nil {
		return model.Flight{}, err
	}

	flight := model.Flight{
		ID:                 id,
		ACRegister:         acRegister,
		ActualFlightTime:   actualFlightTime,
		EstimateFlightTime: estimateFlightTime,
		Canceled:           canceled,
		Delayed:            delayed,
		FlightNumber:       flightNumber,
		ScheduleFlightTime: scheduleFlightTime,
	}

	return flight, nil
}

func (f *FlightRepository) GetFlightByTypeAndSchedule(fn string, flightType string, std string) (model.Flight, error) {
	return f.findFlightForChangeLog(`
		SELECT id, flight_number, schedule_flight_time, COALESCE(ac_register, ''), COALESCE(aircraft, ''), actual_flight_time, estimate_flight_time, COALESCE(canceled, FALSE), COALESCE(delayed, FALSE)
		FROM flight_flight
		WHERE flight_number = $1 AND type = $2 AND schedule_flight_time = $3
		LIMIT 1`, fn, flightType, std)
}

func (f *FlightRepository) GetFlightByTypeAndDate(fn string, flightType string, date string) (model.Flight, error) {
	return f.findFlightForChangeLog(`
		SELECT id, flight_number, schedule_flight_time, COALESCE(ac_register, ''), COALESCE(aircraft, ''), actual_flight_time, estimate_flight_time, COALESCE(canceled, FALSE), COALESCE(delayed, FALSE)
		FROM flight_flight
		WHERE flight_number = $1
		  AND type = $2
		  AND schedule_flight_time >= $3::date
		  AND schedule_flight_time < ($3::date + INTERVAL '1 day')
		ORDER BY schedule_flight_time ASC, id ASC
		LIMIT 1`, fn, flightType, date)
}

func (f *FlightRepository) FindDepartureFlightByDestinationAndDate(flightNumber string, destination string, date string) (model.Flight, error) {
	return f.findFlightForChangeLog(`
		SELECT id, flight_number, schedule_flight_time, COALESCE(ac_register, ''), COALESCE(aircraft, ''), actual_flight_time, estimate_flight_time, COALESCE(canceled, FALSE), COALESCE(delayed, FALSE)
		FROM flight_flight
		WHERE flight_number = $1
		  AND type = 'DEP'
		  AND next_station = $2
		  AND schedule_flight_time >= $3::date
		  AND schedule_flight_time < ($3::date + INTERVAL '1 day')
		ORDER BY schedule_flight_time ASC, id ASC
		LIMIT 1`, flightNumber, destination, date)
}

func (f *FlightRepository) FindArrivalFlightByRouteAndDate(flightNumber string, departure string, destination string, date string) (model.Flight, error) {
	return f.findFlightForChangeLog(`
		SELECT id, flight_number, schedule_flight_time, COALESCE(ac_register, ''), COALESCE(aircraft, ''), actual_flight_time, estimate_flight_time, COALESCE(canceled, FALSE), COALESCE(delayed, FALSE)
		FROM flight_flight
		WHERE flight_number = $1
		  AND type = 'ARR'
		  AND schedule_flight_time >= $4::date
		  AND schedule_flight_time < ($4::date + INTERVAL '2 day')
		  AND (
		    prev_station = $2 OR
		    next_station = $3 OR
		    prev_station = $3 OR
		    next_station = $2
		  )
		ORDER BY schedule_flight_time ASC, id ASC
		LIMIT 1`, flightNumber, departure, destination, date)
}

func (f *FlightRepository) findFlightForChangeLog(query string, args ...interface{}) (model.Flight, error) {
	var (
		id                 int
		flightNumber       string
		scheduleFlightTime *time.Time
		acRegister         string
		aircraftType       string
		actualFlightTime   *time.Time
		estimateFlightTime *time.Time
		canceled           bool
		delayed            bool
	)

	err := f.DB.QueryRow(query, args...).Scan(
		&id,
		&flightNumber,
		&scheduleFlightTime,
		&acRegister,
		&aircraftType,
		&actualFlightTime,
		&estimateFlightTime,
		&canceled,
		&delayed,
	)
	if err != nil {
		return model.Flight{}, err
	}

	return model.Flight{
		ID:                 id,
		ACRegister:         acRegister,
		AircraftType:       aircraftType,
		ActualFlightTime:   actualFlightTime,
		EstimateFlightTime: estimateFlightTime,
		Canceled:           canceled,
		Delayed:            delayed,
		FlightNumber:       flightNumber,
		ScheduleFlightTime: scheduleFlightTime,
	}, nil
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
	ac_register, working, finished, canceled, created_at, updated_at
	) 
	VALUES ($1, $2, $3, $4, $5, $6, $7, false, false, false, now(), now())`)

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
		flight.PrevStation,
		flight.Register)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateDepartureFlight(flightNumber string, date string, datetime string) error {

	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET actual_flight_time=$1 
	WHERE flight_number = $2 and
	type = 'DEP' and
	schedule_flight_time >= $3::date and
	schedule_flight_time < ($3::date + INTERVAL '1 day')`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(datetime, flightNumber, date)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateArrivalFlight(flightNumber string, date string, datetime string) error {

	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET actual_flight_time=$1 
	WHERE flight_number = $2 and
	type = 'ARR' and
	schedule_flight_time >= $3::date and
	schedule_flight_time < ($3::date + INTERVAL '1 day')`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(datetime, flightNumber, date)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateCanceledFlight(flightNumber string, std string) error {
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET canceled=TRUE
	WHERE flight_number = $1 and type = 'DEP' and schedule_flight_time = $2`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(flightNumber, std)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateUncanceledFlight(flightNumber string, std string) error {
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET canceled=FALSE
	WHERE flight_number = $1 and type = 'DEP' and schedule_flight_time = $2`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(flightNumber, std)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateDelayedFlight(flightNumber string, std string) error {
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET delayed=TRUE
	WHERE flight_number = $1 and type = 'DEP' and schedule_flight_time = $2`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(flightNumber, std)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateScheduleFlightTimeByID(flightID int, std string) error {
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET schedule_flight_time=$1
	WHERE id = $2 and type = 'DEP'`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(std, flightID)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateDepartureEstimateFlightBySchedule(flightNumber string, std string, estimate string) error {
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET estimate_flight_time = $1
	WHERE flight_number = $2 and type = 'DEP' and schedule_flight_time = $3`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(estimate, flightNumber, std)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateEstimateFlightByDestination(flightNumber string, destination string, date string, datetime string) error {
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET estimate_flight_time = $1
	WHERE flight_number = $2 and
	type = 'DEP' and
	next_station = $3 and
	schedule_flight_time >= $4::date and
	schedule_flight_time < ($4::date + INTERVAL '1 day')`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(datetime, flightNumber, destination, date)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateArrivalEstimateFlightByRoute(flightNumber string, departure string, destination string, date string, datetime string) error {
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET estimate_flight_time = $1
	WHERE flight_number = $2 and
	type = 'ARR' and
	schedule_flight_time >= $5::date and
	schedule_flight_time < ($5::date + INTERVAL '2 day') and
	(
		prev_station = $3 or
		next_station = $4 or
		prev_station = $4 or
		next_station = $3
	)`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(datetime, flightNumber, departure, destination, date)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateTOBTFlight(flightNumber string, datetime string) error {

	datetime = datetime + "+00"

	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET tobt=$1 
	WHERE flight_number = $2 and 
	type = 'DEP' and
	schedule_flight_time >= CURRENT_DATE and 
	schedule_flight_time <= CURRENT_DATE + INTERVAL '1 day'`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(datetime, flightNumber)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateBay(flightNumber string, std string, bay string) error {
	stmt, err := f.DB.Prepare(`UPDATE flight_flight SET bay=$1 
	WHERE flight_number = $2 and 
	schedule_flight_time >= CURRENT_DATE and 
	schedule_flight_time <= CURRENT_DATE + INTERVAL '1 day'`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(bay, flightNumber)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateRegister(flightNumber string, register string, std string) error {
	stmt, err := f.DB.Prepare(`UPDATE public.flight_flight SET 
		ac_register = $1
		where flight_number = $2 and
		schedule_flight_time = $3`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(register, flightNumber, std)

	if err2 != nil {
		return err2
	}

	return nil
}

func (f *FlightRepository) UpdateAircraft(flightNumber string, aircraft string, std string) error {
	stmt, err := f.DB.Prepare(`UPDATE public.flight_flight SET 
		aircraft = $1
		where flight_number = $2 and
		schedule_flight_time = $3`)

	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err2 := stmt.Exec(aircraft, flightNumber, std)

	if err2 != nil {
		return err2
	}

	return nil
}
