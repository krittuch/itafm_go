package repository

import (
	"database/sql"
	"log"

	"aerothai/itafm/model"
)

type SurveillanceRepositoryInterface interface {
	InsertOrUpdateSurveillance(post *model.AODSSurveillance) bool
	InsertOrUpdateSurveillanceBatch(posts []*model.AODSSurveillance) bool
	InsertSurveillance(post *model.AODSSurveillance) bool
}

type SurveillanceRepository struct {
	DB *sql.DB
}

func NewSurveillanceRepository(db *sql.DB) SurveillanceRepositoryInterface {
	return &SurveillanceRepository{DB: db}
}

func (s *SurveillanceRepository) InsertSurveillance(post *model.AODSSurveillance) bool {
	stmt, err := s.DB.Prepare(`INSERT INTO flight_aircraftlocation (
		callsign,departure,destination,actype,wturbulance,latitude,longitude
		,altitude,gspeed,heading,acaddress,sic,sac,ssrcode
		,datetime,trackno,vx,vy,cdm) 
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,
			$12,$13,$14,$15,$16,$17,$18,$19)`)

	if err != nil {
		log.Println(err)
		return false
	}

	// Close after Function
	defer stmt.Close()

	_, err2 := stmt.Exec(
		post.CallSign,
		post.Departure,
		post.Destination,
		post.AircraftType,
		post.WakeTurbulance,
		post.Lat,
		post.Lon,
		post.Altitude,
		post.GroundSpeed,
		post.Heading,
		post.AircraftAddress,
		post.SIC,
		post.SAC,
		post.SSRCode,
		post.DateTime,
		post.TrackNumber,
		post.VX,
		post.VY,
		post.CDM,
	)

	if err2 != nil {
		log.Println(err2)
		return false
	}

	return true
}

func (s *SurveillanceRepository) InsertOrUpdateSurveillance(post *model.AODSSurveillance) bool {
	stmt, err := s.DB.Prepare(`SELECT COUNT(*) FROM flight_aircraftlocation WHERE callsign = $1`)
	if err != nil {
		log.Println(err)
		return false
	}

	var count int
	err = stmt.QueryRow(post.CallSign).Scan(&count)
	if err != nil {
		log.Println(err)
		return false
	}

	if count == 0 {
		stmt, err = s.DB.Prepare(`INSERT INTO flight_aircraftlocation (
			callsign,departure,destination,actype,wturbulance,latitude,longitude
			,altitude,gspeed,heading,acaddress,sic,sac,ssrcode
			,datetime,trackno,vx,vy,cdm, created_at, updated_at) 
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,
				$12,$13,$14,$15,$16,$17,$18,$19,now(),now())`)
		if err != nil {
			log.Println(err)
			return false
		}

		_, err = stmt.Exec(
			post.CallSign,
			post.Departure,
			post.Destination,
			post.AircraftType,
			post.WakeTurbulance,
			post.Lat,
			post.Lon,
			post.Altitude,
			post.GroundSpeed,
			post.Heading,
			post.AircraftAddress,
			post.SIC,
			post.SAC,
			post.SSRCode,
			post.DateTime,
			post.TrackNumber,
			post.VX,
			post.VY,
			post.CDM,
		)
		if err != nil {
			log.Println(err)
			return false
		}
	} else {
		stmt, err = s.DB.Prepare(`UPDATE flight_aircraftlocation SET
			departure = $2,
			destination = $3,
			actype = $4,
			wturbulance = $5,
			latitude = $6,
			longitude = $7,
			altitude = $8,
			gspeed = $9,
			heading = $10,
			acaddress = $11,
			sic = $12,
			sac = $13,
			ssrcode = $14,
			datetime = $15,
			trackno = $16,
			vx = $17,
			vy = $18,
			cdm = $19,
			updated_at = now()
			WHERE callsign = $1`)
		if err != nil {
			log.Println(err)
			return false
		}

		_, err = stmt.Exec(
			post.CallSign,
			post.Departure,
			post.Destination,
			post.AircraftType,
			post.WakeTurbulance,
			post.Lat,
			post.Lon,
			post.Altitude,
			post.GroundSpeed,
			post.Heading,
			post.AircraftAddress,
			post.SIC,
			post.SAC,
			post.SSRCode,
			post.DateTime,
			post.TrackNumber,
			post.VX,
			post.VY,
			post.CDM,
		)
		if err != nil {
			log.Println(err)
			return false
		}
	}

	return true
}

func (s *SurveillanceRepository) InsertOrUpdateSurveillanceBatch(posts []*model.AODSSurveillance) bool {
	if len(posts) == 0 {
		return true
	}

	tx, err := s.DB.Begin()
	if err != nil {
		log.Println(err)
		return false
	}

	updateStmt, err := tx.Prepare(`UPDATE flight_aircraftlocation SET
		departure = $2,
		destination = $3,
		actype = $4,
		wturbulance = $5,
		latitude = $6,
		longitude = $7,
		altitude = $8,
		gspeed = $9,
		heading = $10,
		acaddress = $11,
		sic = $12,
		sac = $13,
		ssrcode = $14,
		datetime = $15,
		trackno = $16,
		vx = $17,
		vy = $18,
		cdm = $19,
		updated_at = now()
		WHERE callsign = $1`)
	if err != nil {
		log.Println(err)
		_ = tx.Rollback()
		return false
	}
	defer updateStmt.Close()

	insertStmt, err := tx.Prepare(`INSERT INTO flight_aircraftlocation (
		callsign,departure,destination,actype,wturbulance,latitude,longitude
		,altitude,gspeed,heading,acaddress,sic,sac,ssrcode
		,datetime,trackno,vx,vy,cdm, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,
			$12,$13,$14,$15,$16,$17,$18,$19,now(),now())`)
	if err != nil {
		log.Println(err)
		_ = tx.Rollback()
		return false
	}
	defer insertStmt.Close()

	for _, post := range posts {
		if post == nil || post.CallSign == "" {
			continue
		}

		updateResult, err := updateStmt.Exec(
			post.CallSign,
			post.Departure,
			post.Destination,
			post.AircraftType,
			post.WakeTurbulance,
			post.Lat,
			post.Lon,
			post.Altitude,
			post.GroundSpeed,
			post.Heading,
			post.AircraftAddress,
			post.SIC,
			post.SAC,
			post.SSRCode,
			post.DateTime,
			post.TrackNumber,
			post.VX,
			post.VY,
			post.CDM,
		)
		if err != nil {
			log.Println(err)
			_ = tx.Rollback()
			return false
		}

		rowsAffected, err := updateResult.RowsAffected()
		if err != nil {
			log.Println(err)
			_ = tx.Rollback()
			return false
		}

		if rowsAffected > 0 {
			continue
		}

		_, err = insertStmt.Exec(
			post.CallSign,
			post.Departure,
			post.Destination,
			post.AircraftType,
			post.WakeTurbulance,
			post.Lat,
			post.Lon,
			post.Altitude,
			post.GroundSpeed,
			post.Heading,
			post.AircraftAddress,
			post.SIC,
			post.SAC,
			post.SSRCode,
			post.DateTime,
			post.TrackNumber,
			post.VX,
			post.VY,
			post.CDM,
		)
		if err != nil {
			log.Println(err)
			_ = tx.Rollback()
			return false
		}
	}

	if err := tx.Commit(); err != nil {
		log.Println(err)
		return false
	}

	return true
}
