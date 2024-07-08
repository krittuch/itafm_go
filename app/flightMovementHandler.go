package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/go-stomp/stomp/v3"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

func onFPLReceive(
	msg *stomp.Message,
	db *sql.DB,
	flightController *controller.FlightController) {
	fplData := model.FlightPlan{}
	err := json.Unmarshal(msg.Body, &fplData)
	if err != nil {
		log.Println(err)
		return
	}

	r, err2 := regexp.Compile(`(DOF\/)\w+`)

	if err2 == nil {
		// log.Println(r.FindString(data.ITEM18))
		dof := r.FindString(fplData.ITEM18)
		fplData.DOF = strings.Replace(dof, `DOF/`, "", 1)
	} else {
		log.Println("err on regex : $1", err2)
	}

	regex, err3 := regexp.Compile(`(REG\/)\w+`)

	register := ""

	if err3 == nil {
		// log.Println(r.FindString(data.ITEM18))
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

	airlineController := controller.NewAirlineController(db)

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
	icaoCode := airlineCodeRegex.FindString(fplData.CALLSIGN)

	airline, errAirline := airlineController.GetAirline(icaoCode)

	if errAirline != nil {
		// log.Println(errAirline)
		return
	}

	postFlight.FlightNumber = fmt.Sprint(airline.IATA, " ", fplData.CALLSIGN[3:])

	// Choose Flight Type

	if fplData.DEPARTURE == "VTBS" || fplData.DEPARTURE == "VTBD" {
		postFlight.Type = "DEP"
	} else {
		postFlight.Type = "ARR"
	}

	// Create STD
	dateOfFlight := ""

	if fplData.DOF == "" {
		t := time.Now()
		timeString := t.Format("2006-01-02 15:04:05")
		dateOfFlight = strings.Join([]string{strings.Split(timeString, " ")[0], " ", fplData.ETD[:2], ":", fplData.ETD[2:4], ":00"}, "")
	} else {
		dateOfFlight = strings.Join([]string{"20", fplData.DOF}, "")
		if len(dateOfFlight) >= 4 {
			dateOfFlight = dateOfFlight[:4] + "-" + dateOfFlight[4:6] + "-" + dateOfFlight[6:]
			dateOfFlight = strings.Join([]string{dateOfFlight, " ", fplData.ETD[:2], ":", fplData.ETD[2:4], ":00"}, "")
		} else {
			return
		}
	}

	std, errTime := time.Parse("2006-01-02 15:04:05", dateOfFlight)

	if errTime != nil {
		log.Println(errTime)
		return
	}

	postFlight.ScheduleFlightTime = std

	flightController.InsertFlight(&postFlight)

}

func onCMDReceive(
	msg *stomp.Message,
	db *sql.DB,
	flightController *controller.FlightController) {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(msg.Body, &fmvData)
	if err != nil {
		log.Println(err)
		log.Println(string(msg.Body))
		return
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	airlineCodeRegex := regexp.MustCompile(`^[A-Z]{3}`)
	icaoCode := airlineCodeRegex.FindString(fmvData.CALLSIGN)

	airline, errAirline := airlineController.GetAirline(icaoCode)

	if errAirline != nil {
		return
	}

	flightNumber := fmt.Sprint(airline.IATA, " ", fmvData.CALLSIGN[3:])

	// Create ATD
	dateOfFlight := ""
	timeStr := ""

	if fmvData.CMD == "DEP" {
		timeStr = fmvData.TIME1
		dateOfFlight = strings.Join([]string{"20", fmvData.DOF}, "")
		if len(dateOfFlight) < 4 {
			log.Println("Error DEP CMD", dateOfFlight)
			return
		}
		dateOfFlight = dateOfFlight[:4] + "-" + dateOfFlight[4:6] + "-" + dateOfFlight[6:]
		dateOfFlight = strings.Join([]string{dateOfFlight, " ", timeStr[:2], ":", timeStr[2:4], ":00+00"}, "")
	} else if fmvData.CMD == "ARR" {
		timeStr = fmvData.TIME2
		t := time.Now()
		timeString := t.Format("2006-01-02 15:04:05")
		dString := strings.Split(timeString, " ")[0]

		fmvData.DOF = dString
		dateOfFlight = strings.Join([]string{
			dString, " ",
			timeStr[:2], ":",
			timeStr[2:4], ":00+00",
		}, "")
	} else {
		return
	}

	flightController.UpdateDepartureFlight(flightNumber, fmvData.DOF, dateOfFlight)
}

func onCNLReceive(
	msg *stomp.Message,
	db *sql.DB,
	flightController *controller.FlightController) {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(msg.Body, &fmvData)
	if err != nil {
		log.Println(err)
		log.Println(string(msg.Body))
		return
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	airline, errAirline := airlineController.GetAirline(fmvData.CALLSIGN[:3])

	if errAirline != nil {
		log.Println("Could not find airline")
		log.Println(errAirline)
		return
	}

	flightNumber := fmt.Sprint(airline.IATA, " ", fmvData.CALLSIGN[3:])

	log.Println(string(msg.Body))
	log.Println(flightNumber)

	// Create ATD
	// dateOfFlight := ""
	// timeStr := ""

	// flightController.UpdateDepartureFlight(flightNumber, fmvData.DOF, dateOfFlight)
}

func onDLYReceive(
	msg *stomp.Message,
	db *sql.DB,
	flightController *controller.FlightController) {
	fmvData := model.AODSFlightMovement{}
	err := json.Unmarshal(msg.Body, &fmvData)
	if err != nil {
		log.Println(err)
		log.Println(string(msg.Body))
		return
	}

	// Change icao to iata

	airlineController := controller.NewAirlineController(db)

	airline, errAirline := airlineController.GetAirline(fmvData.CALLSIGN[:3])

	if errAirline != nil {
		log.Println("Could not find airline")
		log.Println(errAirline)
		return
	}

	flightNumber := fmt.Sprint(airline.IATA, " ", fmvData.CALLSIGN[3:])

	log.Println(string(msg.Body))
	log.Println(flightNumber)

	// Create ATD
	// dateOfFlight := ""
	// timeStr := ""

	// flightController.UpdateDepartureFlight(flightNumber, fmvData.DOF, dateOfFlight)
}
