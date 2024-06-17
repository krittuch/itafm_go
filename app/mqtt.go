package app

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/go-stomp/stomp/v3"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

var serverAddr = flag.String("server", MQTT_IP_ADDRESS+":"+MQTT_PORT, "AODS server endpoint")
var topicFLMOName = flag.String("flmtopic", MQTT_FLIGHT_MOVEMENT_TOPIC, "FLMO Topic")
var topicIDEPName = flag.String("ideptopic", MQTT_IDEP_TOPIC, "IDEP Topic")
var stop = make(chan bool)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login(MQTT_USER, MQTT_PASSWORD),
	stomp.ConnOpt.Host("/"),
	stomp.ConnOpt.HeartBeat(30, 30),
	stomp.ConnOpt.HeartBeatError(360 * time.Second),
}

func StartConnectMQTT(a *App) {
	flag.Parse()
	subFlight := make(chan bool)
	subIDEP := make(chan bool)
	log.Println(serverAddr)
	conn, err := stomp.Dial("tcp", *serverAddr, options...)

	if err != nil {
		log.Panicln(err)
		return
	}
	go recvIDEPMessages(subIDEP, a.DB, conn)
	go recvFltMessages(subFlight, a.DB, conn)

	select {}
	// <-subscribed

	// <-stop
	log.Println("Stop MQTT Message")
}

func recvIDEPMessages(_ chan bool, db *sql.DB, conn *stomp.Conn) {
	defer func() {
		stop <- true
	}()

	sub, err := conn.Subscribe(*topicIDEPName, stomp.AckAuto)

	if err != nil {
		log.Println("cannot subscribe to", *topicIDEPName, err.Error())
		return
	}

	log.Println("Connect To iDEP")
	// flightController := controller.NewFlightController(db)

	for {
		msg := <-sub.C

		if len(msg.Body) <= 0 {
			log.Println(msg.Body)
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
		log.Println(string(msg.Body))

		bay := data.DepartureParkingStand
		if bay == "" {
			bay = data.ArrivalParkingStand
		}

		if bay == "" {
			continue
		}

		airlineController := controller.NewAirlineController(db)

		airline, errAirline := airlineController.GetAirline(data.AircraftID[:3])

		if errAirline != nil {
			log.Println(errAirline)
			return
		}

		flightNumber := fmt.Sprint(airline.IATA, " ", data.AircraftID[3:])

		fltCtl := controller.NewFlightController(db)

		fltCtl.UpdateBay(flightNumber, data.EOBT, bay)

	}

}

func recvFltMessages(_ chan bool, db *sql.DB, conn *stomp.Conn) {
	defer func() {
		stop <- true
	}()

	sub, err := conn.Subscribe(*topicFLMOName, stomp.AckAuto)
	if err != nil {
		log.Println("cannot subscribe to", *topicFLMOName, err.Error())
		return
	}

	log.Println("Connect to Flight Movement")

	flightController := controller.NewFlightController(db)

	for {
		msg := <-sub.C

		if len(msg.Body) <= 0 {
			log.Println(msg.Body)
			log.Println("Message is Empty")
			continue
		}

		data := model.AODSFlightMovement{}
		err := json.Unmarshal(msg.Body, &data)

		if err != nil {
			log.Println("Erro  on Flight Movement mqtt")
			log.Println(err)
			log.Println(string(msg.Body))
			continue
		}

		if data.CMD == "FPL" {
			onFPLReceive(msg, db, flightController)
		} else {
			onCMDReceive(msg, db, flightController)
		}
	}

}
