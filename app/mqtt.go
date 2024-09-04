package app

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/go-stomp/stomp/v3"
	"github.com/gocarina/gocsv"
	mqtt "github.com/eclipse/paho.mqtt.golang"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"


)

var serverAddr = flag.String("server", MQTT_IP_ADDRESS+":"+MQTT_PORT, "AODS server endpoint")
var itafmServerAddr = flag.String("itafmserver", ITAFM_MQTT_IP_ADDRESS+":"+ITAFM_MQTT_PORT, "AODS server endpoint")
var topicFLMOName = flag.String("flmtopic", MQTT_FLIGHT_MOVEMENT_TOPIC, "FLMO Topic")
var topicIDEPName = flag.String("ideptopic", MQTT_IDEP_TOPIC, "IDEP Topic")
var topicSURVName = flag.String("survtopic", MQTT_SURV_TOPIC, "SURV Topic")
var itafmSurvTopicName = flag.String("itafmsurvtopic", ITAFM_SURV_TOPIC, "SURV Topic")
var itafmFlightTopicName = flag.String("itafmflighttopic", ITAFM_SURV_TOPIC, "SURV Topic")
var stop = make(chan bool)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login(MQTT_USER, MQTT_PASSWORD),
	stomp.ConnOpt.Host("/"),
	stomp.ConnOpt.HeartBeat(120, 120),
	stomp.ConnOpt.HeartBeatError(360 * time.Second),
	// stomp.ConnOpt.RcvReceiptTimeout(360 * time.Second),
}

var airlines []*model.CSVAirline

func StartConnectMQTT(a *App) {

	in, err := os.Open("data/flight_airlinecode.csv")
	if err != nil {
		panic(err)
	}
	defer in.Close()

	airlines = []*model.CSVAirline{}

	if err := gocsv.UnmarshalFile(in, &airlines); err != nil {
		panic(err)
	}

		flag.Parse()
		// subFlight := make(chan bool)
		

		// subscribe := make
		

		// Create connection to itafm mqtt
		client := initITAFM()

		if token := client.Connect(); token.Wait() && token.Error() != nil {
			log.Println(token.Error())
			return
		}

		log.Println("Connected To AODS Server")
		
		// go recvFltMessages(subFlight, a.DB, client)
		connectToSurveillance(a.DB, client)
		connectToIDEP(a.DB, client)
		connectToFLT(a.DB, client)

		// Listen for a stop signal to break the loop and end the function

		select {}
		<-stop
		log.Println("Stop MQTT Message")
		

}

func connectToSurveillance (db *sql.DB, client mqtt.Client) {
	subSurv := make(chan bool)
	go recvSurvMessages(subSurv, db, client)
}

func connectToIDEP (db *sql.DB, client mqtt.Client) {
	subIDEP := make(chan bool)
	go recvIDEPMessages(subIDEP, db, client)
}

func connectToFLT(db *sql.DB, client mqtt.Client) {
	subFlight := make(chan bool)
	go recvFltMessages(subFlight, db, client)
}


// Change Flight number from ICAO to IATA
// Such as THA616 to TG 616
func ConvertToIATA(flightNumber string) (string, bool) {
	if len(flightNumber) < 3 {
		log.Println("Flight length lower than 3")
		return flightNumber, false
	}

	icaoCode := flightNumber[:3]

	for _, airline := range airlines {
		if airline.ICAO == icaoCode {
			return (airline.IATA + flightNumber[3:]), true
		}
	}

	log.Println("Cannot find ", icaoCode)

	return flightNumber, false
}

func recvSurvMessages(_ chan bool, db *sql.DB, client mqtt.Client) {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)

	if err != nil {
		println("cannot connect to server", err.Error())
		return
	}

	sub, err := conn.Subscribe(*topicSURVName, stomp.AckAuto)

	if err != nil {
		log.Println("cannot subscribe to", *topicSURVName, err.Error())
		return
	}

	log.Println("Connect to Surveillance")

	for {

		msg := <-sub.C

		if msg == nil {
			continue
		}

		if len(msg.Body) <= 0 {
			log.Println(msg.Body)
			log.Println("Message is Empty")
			conn.Disconnect()
			connectToSurveillance(db, client)
			return
		}

		if msg.Err != nil {
			log.Println("Message Error from Surveillance")
			log.Println(msg.Err)
			
			continue
		}

		survController := controller.NewSurveillanceController(db)

		onSurveillanceReceive(msg, db, survController, client)
	}
}

func recvIDEPMessages(_ chan bool, db *sql.DB, client mqtt.Client) {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)

	if err != nil {
		println("cannot connect to server", err.Error())
		return
	}

	sub, err := conn.Subscribe(*topicIDEPName, stomp.AckAuto)

	if err != nil {
		log.Println("cannot subscribe to", *topicIDEPName, err.Error())
		return
	}

	log.Println("Connect To iDEP")
	flightController := controller.NewFlightController(db)

	for {
		msg := <-sub.C
		
		if msg == nil {
			continue
		}

		if len(msg.Body) <= 0 {
			log.Println(msg.Body)
			log.Println("Message is Empty")
			conn.Disconnect()
			connectToIDEP(db, client)
			return
		}

		if msg.Err != nil {
			log.Println("Message Error from IDEP")
			log.Println(msg.Err)
			continue
		}

		onIDEPReceive(msg, db, flightController, client)

	}

}

func recvFltMessages(_ chan bool, db *sql.DB,  client mqtt.Client) {
	defer func() {
		stop <- true
	}()

	conn, err := stomp.Dial("tcp", *serverAddr, options...)

	if err != nil {
		println("cannot connect to server", err.Error())
		return
	}

	sub, err := conn.Subscribe(*topicFLMOName, stomp.AckAuto)
	if err != nil {
		log.Println("cannot subscribe to", *topicFLMOName, err.Error())
		return
	}

	flightController := controller.NewFlightController(db)

	for {
		msg := <-sub.C

		if msg == nil {
			continue
		}

		if len(msg.Body) <= 0 {
			log.Println(msg.Body)
			log.Println("Message is Empty")
			conn.Disconnect()
			connectToFLT(db, client)
			return
		}

		if msg.Err != nil {
			log.Println("Message Error from FLT Plan")
			log.Println(msg.Err)
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
			onFPLReceive(msg, db, flightController, client)
		} 
		// else if data.CMD == "DEP" || data.CMD == "ARR" {
		// 	onCMDReceive(msg, db, flightController)
		// } else if data.CMD == "CNL" {
		// 	onCNLReceive(msg, db, flightController)
		// } else if data.CMD == "DLY" {
		// 	onDLYReceive(msg, db, flightController)
		// }
	}
}
