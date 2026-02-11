package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

func onFPLReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) bool {
	fplData := model.FlightPlan{}
	err := json.Unmarshal(body, &fplData)
	if err != nil {
		log.Println(err)
		return false
	}

	r, err2 := regexp.Compile(`(DOF\/)\w+`)

	if err2 == nil {
		dof := r.FindString(fplData.ITEM18)
		fplData.DOF = strings.Replace(dof, `DOF/`, "", 1)
	} else {
		log.Println("err on regex : $1", err2)
	}

	regex, err3 := regexp.Compile(`(REG\/)\w+`)

	register := ""

	if err3 == nil {
		register = regex.FindString(fplData.ITEM18)
		register = strings.Replace(register, `REG/`, "", 1)
	} else {
		log.Println("err on regex : $1", err3)
	}

	postFlight := model.PostFlight{
		AircraftType: fplData.ACTYPE,
		NextStation:  fplData.DESTINATION,
		PrevStation:  fplData.DEPARTURE,
		Register:     register,
	}

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
	icaoCode := airlineCodeRegex.FindString(fplData.CALLSIGN)

	iata, success := ConvertToIATA(icaoCode)

	if !success {
		return false
	}

	numberRegex := regexp.MustCompile(`\d+`)
	matchString := numberRegex.FindString(fplData.CALLSIGN)
	if len(matchString) <= 0 {
		log.Println("Cannot find number in ", fplData.CALLSIGN)
		return false
	}

	flightNumber := strings.TrimLeft(matchString, "0")

	timeStr := fplData.ETD

	dateOfFlight := "20" + fplData.DOF[:2] + "-" + fplData.DOF[2:4] + "-" + fplData.DOF[4:]
	std := strings.Join([]string{dateOfFlight, " ", timeStr[:2], ":", timeStr[2:4], ":00+00"}, "")

	postFlight.FlightNumber = fmt.Sprint(iata, " ", flightNumber)

	flightController.UpdateRegister(postFlight.FlightNumber, postFlight.Register, std)

	flight, err := flightController.GetFlight(postFlight.FlightNumber, std)

	if err != nil {
		log.Println("Cannot find flight : ", postFlight.FlightNumber, std, err)
	} else {

		flightChangeLogController := controller.NewFlightChangeLogController(db)

		err = flightChangeLogController.Insert(model.PostFlightChangeLog{
			FlightID: uint(flight.ID),
			Field:    "ac_register",
			OldValue: flight.ACRegister,
			NewValue: postFlight.Register,
		})

		if err != nil {
			log.Println("FlightCon Insert error of Flight log ac_register change: ", flight.ID, flight.ScheduleFlightTime, err)
		}
	}

	return true
}

func onCMDReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) bool {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(body, &fmvData)
	if err != nil {
		log.Println(err)
		return false
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
	icaoCode := airlineCodeRegex.FindString(fmvData.CALLSIGN)

	airline, errAirline := airlineController.GetAirline(icaoCode)

	if errAirline != nil {
		return false
	}

	flightNumber := fmt.Sprint(airline.IATA, " ", fmvData.CALLSIGN[3:])

	// Create ATD
	dateOfFlight := ""
	timeStr := ""
	std := ""

	if fmvData.CMD == "DEP" {
		timeStr = fmvData.TIME1
		dateOfFlight = strings.Join([]string{"20", fmvData.DOF}, "")
		if len(dateOfFlight) < 4 {
			log.Println("Error DEP CMD", dateOfFlight)
			return false
		}
		dateOfFlight = dateOfFlight[:4] + "-" + dateOfFlight[4:6] + "-" + dateOfFlight[6:]
		std = strings.Join([]string{dateOfFlight, " ", timeStr[:2], ":", timeStr[2:4], ":00+00"}, "")
	} else if fmvData.CMD == "ARR" {
		timeStr = fmvData.TIME2
		t := time.Now()
		timeString := t.Format("2006-01-02 15:04:05")
		dString := strings.Split(timeString, " ")[0]

		fmvData.DOF = dString
		std = strings.Join([]string{
			dString, " ",
			timeStr[:2], ":",
			timeStr[2:4], ":00+00",
		}, "")
	} else {
		return false
	}

	if fmvData.CMD == "DEP" {
		flightController.UpdateDepartureFlight(flightNumber, fmvData.DOF, std)
	} else if fmvData.CMD == "ARR" {
		flightController.UpdateArrivalFlight(flightNumber, fmvData.DOF, std)
	}

	return true
}

func onCNLReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(body, &fmvData)
	if err != nil {
		log.Println(err)
		return
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	if _, errAirline := airlineController.GetAirline(fmvData.CALLSIGN[:3]); errAirline != nil {
		return
	}

	// Create ATD
	// dateOfFlight := ""
	// timeStr := ""

	// flightController.UpdateDepartureFlight(flightNumber, fmvData.DOF, dateOfFlight)
}

func onDLYReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(body, &fmvData)
	if err != nil {
		log.Println(err)
		return
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	if _, errAirline := airlineController.GetAirline(fmvData.CALLSIGN[:3]); errAirline != nil {
		return
	}

}

func onCHGReceive(
	body []byte,
	db *sql.DB,
	flightController *controller.FlightController) {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(body, &fmvData)
	if err != nil {
		log.Println(err)
		return
	}

	airlineController := controller.NewAirlineController(db)

	if _, errAirline := airlineController.GetAirline(fmvData.CALLSIGN[:3]); errAirline != nil {
		return
	}
}
