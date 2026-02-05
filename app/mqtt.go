package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gocarina/gocsv"
	"github.com/segmentio/kafka-go"
)

var kafkaBrokers = flag.String("kafka-brokers", KAFKA_BROKERS, "Kafka broker endpoints separated by comma")
var kafkaGroupID = flag.String("kafka-group", KAFKA_GROUP_ID, "Kafka consumer group id")
var kafkaFlightTopic = flag.String("kafka-flight-topic", KAFKA_FLIGHT_TOPIC, "Kafka topic for flight movement")
var kafkaIDEPTopic = flag.String("kafka-idep-topic", KAFKA_IDEP_TOPIC, "Kafka topic for IDEP")
var kafkaSURVTopic = flag.String("kafka-surv-topic", KAFKA_SURV_TOPIC, "Kafka topic for surveillance")
var itafmSurvTopicName = flag.String("itafm-surv-topic", ITAFM_SURV_TOPIC, "iTAFM surveillance topic")

var airlines []*model.CSVAirline

func StartConsumeKafka(a *App) {
	loadAirlineReference()
	flag.Parse()

	client := initITAFM()
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Println(token.Error())
		return
	}

	log.Println("Connected to iTAFM")

	brokers := splitBrokers(*kafkaBrokers)
	if len(brokers) == 0 {
		log.Println("no Kafka brokers configured")
		return
	}

	go consumeSurveillanceStream(brokers, *kafkaGroupID, *kafkaSURVTopic, a.DB, client)
	go consumeIDEPStream(brokers, *kafkaGroupID, *kafkaIDEPTopic, a.DB, client)
	go consumeFlightStream(brokers, *kafkaGroupID, *kafkaFlightTopic, a.DB, client)

	select {}
}

func loadAirlineReference() {
	in, err := os.Open("data/flight_airlinecode.csv")
	if err != nil {
		panic(err)
	}
	defer in.Close()

	airlines = []*model.CSVAirline{}
	if err := gocsv.UnmarshalFile(in, &airlines); err != nil {
		panic(err)
	}
}

func consumeSurveillanceStream(brokers []string, groupID string, topic string, db *sql.DB, client mqtt.Client) {
	runKafkaConsumerLoop(brokers, groupID, topic, func(value []byte) {
		survController := controller.NewSurveillanceController(db)
		onSurveillanceReceive(value, db, survController, client)
	})
}

func consumeIDEPStream(brokers []string, groupID string, topic string, db *sql.DB, client mqtt.Client) {
	flightController := controller.NewFlightController(db)
	runKafkaConsumerLoop(brokers, groupID, topic, func(value []byte) {
		onIDEPReceive(value, db, flightController, client)
	})
}

func consumeFlightStream(brokers []string, groupID string, topic string, db *sql.DB, client mqtt.Client) {
	flightController := controller.NewFlightController(db)
	runKafkaConsumerLoop(brokers, groupID, topic, func(value []byte) {
		data := model.AODSFlightMovement{}
		err := json.Unmarshal(value, &data)
		if err != nil {
			log.Println("error decoding flight payload:", err)
			return
		}

		switch data.CMD {
		case "FPL":
			onFPLReceive(value, db, flightController, client)
		case "DEP", "ARR":
			onCMDReceive(value, db, flightController, client)
		case "CNL":
			onCNLReceive(value, db, flightController)
		case "DLY":
			onDLYReceive(value, db, flightController)
		default:
			log.Println("ignored flight command:", data.CMD)
		}
	})
}

func runKafkaConsumerLoop(brokers []string, groupID string, topic string, handle func([]byte)) {
	for {
		err := consumeTopic(brokers, groupID, topic, handle)
		log.Printf("consumer for topic %s failed: %v", topic, err)
		time.Sleep(3 * time.Second)
	}
}

func consumeTopic(brokers []string, groupID string, topic string, handle func([]byte)) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	ctx := context.Background()
	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			return err
		}
		if len(msg.Value) == 0 {
			continue
		}
		handle(msg.Value)
	}
}

func splitBrokers(raw string) []string {
	parts := strings.Split(raw, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			brokers = append(brokers, trimmed)
		}
	}
	return brokers
}

// Change flight number from ICAO to IATA, for example THA616 -> TG616.
func ConvertToIATA(flightNumber string) (string, bool) {
	if len(flightNumber) < 3 {
		return flightNumber, false
	}

	icaoCode := flightNumber[:3]
	for _, airline := range airlines {
		if airline.ICAO == icaoCode {
			return airline.IATA + flightNumber[3:], true
		}
	}

	return flightNumber, false
}
