package repository

import (
	"database/sql"
	"time"

	"aerothai/itafm/model"
)

type FlightChangeLogRepository struct {
	DB *sql.DB
}

func NewFlightChangeLogRepository(db *sql.DB) FlightChangeLogRepositoryInterface {
	return &FlightChangeLogRepository{
		DB: db,
	}
}

func (f *FlightChangeLogRepository) FindAll() ([]model.FlightChangeLog, error) {
	rows, err := f.DB.Query("SELECT * FROM flight_flightchangelog")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	flightChangeLogs := []model.FlightChangeLog{}

	for rows.Next() {
		var flightChangeLog model.FlightChangeLog
		err := rows.Scan(&flightChangeLog.ID, &flightChangeLog.FlightID, &flightChangeLog.Field, &flightChangeLog.OldValue, &flightChangeLog.NewValue, &flightChangeLog.CreatedAt, &flightChangeLog.UpdatedAt)
		if err != nil {
			return nil, err
		}
		flightChangeLogs = append(flightChangeLogs, flightChangeLog)
	}
	return flightChangeLogs, nil
}

func (f *FlightChangeLogRepository) FindByFlightID(flightID uint) (model.FlightChangeLog, error) {

	flightChangeLog := model.FlightChangeLog{}

	err := f.DB.QueryRow(`SELECT * FROM flight_flightchangelog WHERE flight_id=$1`,
		flightID).Scan(
		&flightChangeLog.ID,
		&flightChangeLog.FlightID,
		&flightChangeLog.Field,
		&flightChangeLog.OldValue,
		&flightChangeLog.NewValue,
		&flightChangeLog.CreatedAt,
		&flightChangeLog.UpdatedAt,
	)

	if err != nil {
		return model.FlightChangeLog{}, err
	}

	return flightChangeLog, nil
}

func (f *FlightChangeLogRepository) Insert(postFlightChangeLog model.PostFlightChangeLog) error {

	stmt, err := f.DB.Prepare(`
		INSERT INTO flight_flightchangelog (flight_id, field, old_value, new_value, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6);
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	nowUTC := time.Now().UTC()

	_, err = stmt.Exec(
		postFlightChangeLog.FlightID,
		postFlightChangeLog.Field,
		postFlightChangeLog.OldValue,
		postFlightChangeLog.NewValue,
		nowUTC,
		nowUTC)
	if err != nil {
		return err
	}

	return nil

}
