package app

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"time"

	"github.com/go-stomp/stomp/v3"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"
)

var serverAddr = flag.String("server", MQTT_IP_ADDRESS+":"+MQTT_PORT, "AODS server endpoint")
var itafmServerAddr = flag.String("itafmserver", ITAFM_MQTT_IP_ADDRESS+":"+ITAFM_MQTT_PORT, "AODS server endpoint")
var topicFLMOName = flag.String("flmtopic", MQTT_FLIGHT_MOVEMENT_TOPIC, "FLMO Topic")
var topicIDEPName = flag.String("ideptopic", MQTT_IDEP_TOPIC, "IDEP Topic")
var topicSURVName = flag.String("survtopic", MQTT_SURV_TOPIC, "SURV Topic")
var itafmSurvTopicName = flag.String("itafmsurvtopic", ITAFM_SURV_TOPIC, "SURV Topic")
var stop = make(chan bool)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login(MQTT_USER, MQTT_PASSWORD),
	stomp.ConnOpt.Host("/"),
	stomp.ConnOpt.HeartBeat(30, 30),
	stomp.ConnOpt.HeartBeatError(360 * time.Second),
}

var itafmOptions []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login(ITAFM_MQTT_USER, ITAFM_MQTT_PASSWORD),
	stomp.ConnOpt.Host("/"),
	stomp.ConnOpt.HeartBeat(30, 30),
	stomp.ConnOpt.HeartBeatError(360 * time.Second),
}

func StartConnectMQTT(a *App) {
	for {
		flag.Parse()
		// subFlight := make(chan bool)
		// subIDEP := make(chan bool)
		subSurv := make(chan bool)
		conn, err := stomp.Dial("tcp", *serverAddr, options...)

		if err != nil {
			log.Println("Failed to connect to MQTT server:", err)
			log.Println("Attempting to reconnect in 5 minutes...")
			time.Sleep(5 * time.Minute)
			continue
		}

		// go recvIDEPMessages(subIDEP, a.DB, conn)
		// go recvFltMessages(subFlight, a.DB, conn)
		go recvSurvMessages(subSurv, a.DB, conn)

		// Listen for a stop signal to break the loop and end the function
		<-stop
		log.Println("Stop MQTT Message")
		return
	}
}

func recvSurvMessages(_ chan bool, db *sql.DB, conn *stomp.Conn) {
	defer func() {
		stop <- true
	}()

	sub, err := conn.Subscribe(*topicSURVName, stomp.AckAuto)

	if err != nil {
		log.Println("cannot subscribe to", *topicSURVName, err.Error())
		return
	}

	// Create connection to itafm mqtt
	client := initITAFM()

	for {
		msg := <-sub.C

		if len(msg.Body) <= 0 {
			log.Println(msg.Body)
			log.Println("Message is Empty")
			continue
		}

		survController := controller.NewSurveillanceController(db)

		onSurveillanceReceive(msg, db, survController)

		// Also Send to itafm
		sendToITAFM(client, *itafmSurvTopicName, string(msg.Body))
	}
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
	flightController := controller.NewFlightController(db)

	for {
		msg := <-sub.C

		if len(msg.Body) <= 0 {
			log.Println(msg.Body)
			log.Println("Message is Empty")
			continue
		}

		onIDEPReceive(msg, db, flightController)

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
		} else if data.CMD == "DEP" || data.CMD == "ARR" {
			onCMDReceive(msg, db, flightController)
		} else if data.CMD == "CNL" {
			onCNLReceive(msg, db, flightController)
		} else if data.CMD == "DLY" {
			onDLYReceive(msg, db, flightController)
		}
	}
}
