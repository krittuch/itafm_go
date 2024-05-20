package app

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"regexp"
	"strings"

	"github.com/go-stomp/stomp/v3"

	"aerothai/itafm/model"
)

const defaultPort = ":61613"

var serverAddr = flag.String("server", "172.16.21.223:61613", "AODS server endpoint")
var queueFLMOName = flag.String("flmqueue", "/queue/FLMO_ITFM_Queue", "FLMO Queue")
var topicSvlName = flag.String("topic", "/topic/AS62_VTBB_ITFM_Topic", "Survillance Topic")
var queueIDEPName = flag.String("idepqueue", "/queue/IDEP_ITFM_Queue", "IDEP Queue")
var stop = make(chan bool)

var options []func(*stomp.Conn) error = []func(*stomp.Conn) error{
	stomp.ConnOpt.Login("itfm", "itfm"),
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
		data := model.IDEP{}
		err := json.Unmarshal(msg.Body, &data)

		log.Println("IDEP MSG Receive")

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

	// flightController := controller.NewFlightController(db)

	for {
		msg := <-sub.C

		data := model.Command{}
		err := json.Unmarshal(msg.Body, &data)

		if err != nil {
			log.Println("Flight Movement")
			log.Println(err)
			log.Println(string(msg.Body))
			continue
		}

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

			// fplctrl.InsertJSONFlightPlan(&fplData)
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
