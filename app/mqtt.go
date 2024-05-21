package app

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/go-stomp/stomp/v3"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

var serverAddr = flag.String("server", MQTT_IP_ADDRESS+":"+MQTT_PORT, "AODS server endpoint")
var queueFLMOName = flag.String("flmtopic", MQTT_FLIGHT_MOVEMENT_TOPIC, "FLMO Topic")
var queueIDEPName = flag.String("ideptopic", MQTT_IDEP_TOPIC, "IDEP Topic")
var stop = make(chan bool)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login(MQTT_USER, MQTT_PASSWORD),
	stomp.ConnOpt.Host("/"),
	stomp.ConnOpt.HeartBeat(30, 30),
}

func StartConnectMQTT(a *App) {
	flag.Parse()
	subFlight := make(chan bool)
	subIDEP := make(chan bool)
	go recvIDEPMessages(subIDEP, a.DB)
	go recvFltMessages(subFlight, a.DB)

	select {}
	// <-subscribed

	// <-stop
	log.Println("Stop MQTT Message")
}

func recvIDEPMessages(subscribe chan bool, db *sql.DB) {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)


	if err != nil {
		log.Println("cannot connect to server", err.Error())
		return
	}

	sub, err := conn.Subscribe(*queueIDEPName, stomp.AckAuto)
	
	if err != nil {
		log.Println("cannot subscribe to", *queueIDEPName, err.Error())
		return
	}

	log.Println("Connect To iDEP")
	// flightController := controller.NewFlightController(db)

	for {
		msg := <-sub.C

		if len(msg.Body) <= 0 {
			log.Println("Message is Empty")
			continue
		}

		data := model.IDEP{}
		err := json.Unmarshal(msg.Body, &data)

		if err != nil {
			log.Println("Error IDEP")
			log.Println(err)
			log.Println(string(msg.Body))
			continue
		}

		log.Println("IDEP Receive")

	}

}

func recvFltMessages(subscribed chan bool, db *sql.DB) {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)

	if err != nil {
		log.Println("cannot connect to server", err.Error())
		return
	}

	sub, err := conn.Subscribe(*queueFLMOName, stomp.AckAuto)
	if err != nil {
		log.Println("cannot subscribe to", *queueFLMOName, err.Error())
		return
	}

	log.Println("Connect to Flight Movement")

	flightController := controller.NewFlightController(db)

	for {
		msg := <-sub.C

		if len(msg.Body) <= 0 {
			log.Println("Message is Empty")
			continue
		}

		data := model.AODSFlightMovement{}
		err := json.Unmarshal(msg.Body, &data)


		if err != nil {
			log.Println("Flight Movement")
			log.Println(err)
			log.Println(string(msg.Body))
			continue
		}

		log.Println(string(msg.Body))


		if data.CMD == "FPL" {
			fplData := model.FlightPlan{}
			err := json.Unmarshal(msg.Body, &fplData)
			if err != nil {
				log.Println(err)
				log.Println(string(msg.Body))
				continue
			}
			r, err2 := regexp.Compile(`(DOF\/)\w+`)

			if err2 == nil {
				// log.Println(r.FindString(data.ITEM18))
				dof := r.FindString(fplData.ITEM18)
				fplData.DOF = strings.Replace(dof, `DOF/`, "", 1)
			} else {
				log.Println("err on regex : $1", err2)
			}

			// Used parameter : CALLSIGN, ACTYPE, ETD, DEPARTURE, DESTINATION
			// Convert to flight model :
			// CALLSIGN -> FlightNumber (With iata code)
			// ACTYPE -> AircraftType (Cut prefix 1 char)
			// DOF + ETD -> ScheduleFlightTime
			// DEPARTURE -> PrevStation
			// DESTINATION -> NextStation
			// if DEPARTURE == VTBS then Type = DEP else Type = ARR
			//
			postFlight := model.PostFlight{
				AircraftType: fplData.ACTYPE,
				NextStation:  fplData.DESTINATION,
				PrevStation:  fplData.DEPARTURE,
			}

			// Change icao to iata

			airlineController := controller.NewAirlineController(db)

			airline, errAirline := airlineController.GetAirline(fplData.CALLSIGN[:3])

			if errAirline != nil {
				log.Println(errAirline)
				continue
			}

			postFlight.FlightNumber = fmt.Sprint(airline.IATA, " ", fplData.CALLSIGN[3:])

			// Choose Flight Type

			if fplData.DEPARTURE == "VTBS" {
				postFlight.Type = "DEP"
			} else {
				postFlight.Type = "ARR"
			}

			// Create STD
			dateOfFlight := ""

			if fplData.DOF == "" {
				t := time.Now()
				timeString := t.Format("2006-01-02 15:04:05")
				dateOfFlight = fmt.Sprint(strings.Split(timeString, " ")[0], " ", fplData.ETD, "Z")
			} else {
				dateOfFlight = fmt.Sprint("20", fplData.DOF)
				dateOfFlight = dateOfFlight[:4] + "-" + dateOfFlight[4:6] + "-" + dateOfFlight[6:]
				dateOfFlight = fmt.Sprint(dateOfFlight, " ", fplData.ETD, "Z")
			}

			std, errTime := time.Parse("2006-01-02 15:04:05", dateOfFlight)

			if errTime != nil {
				log.Println(errTime)
				continue
			}

			postFlight.ScheduleFlightTime = std

			flightController.InsertFlight(&postFlight)

			log.Println(fplData)
		} else {
			fmvData := model.AODSFlightMovement{}
			err := json.Unmarshal(msg.Body, &fmvData)
			if err != nil {
				log.Println(err)
				log.Println(string(msg.Body))
				continue
			}

			// fmvctrl.InsertJSONFlightMovement(&fmvData)
			log.Println(fmvData)
		}
	}

}
