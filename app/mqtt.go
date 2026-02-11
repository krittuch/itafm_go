package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"aerothai/itafm/controller"
	"aerothai/itafm/model"

	"github.com/gocarina/gocsv"
	"github.com/segmentio/kafka-go"
)

var kafkaBrokers = flag.String("kafka-brokers", KAFKA_BROKERS, "Kafka broker endpoints separated by comma")
var kafkaGroupID = flag.String("kafka-group", KAFKA_GROUP_ID, "Kafka consumer group id")
var kafkaFlightTopic = flag.String("kafka-flight-topic", KAFKA_FLIGHT_TOPIC, "Kafka topic for flight movement")
var kafkaIDEPTopic = flag.String("kafka-idep-topic", KAFKA_IDEP_TOPIC, "Kafka topic for IDEP")
var kafkaSURVTopic = flag.String("kafka-surv-topic", KAFKA_SURV_TOPIC, "Kafka topic for surveillance")

var airlines []*model.CSVAirline

var errEmptyFlightPayload = errors.New("empty flight payload")

func StartConsumeKafka(a *App) {
	loadAirlineReference()
	flag.Parse()

	brokers := splitBrokers(*kafkaBrokers)
	if len(brokers) == 0 {
		log.Println("no Kafka brokers configured")
		return
	}

	monitor := newGatewayMonitor(*kafkaFlightTopic, *kafkaIDEPTopic, *kafkaSURVTopic)
	monitor.start()

	go consumeSurveillanceStream(brokers, *kafkaGroupID, *kafkaSURVTopic, a.DB, monitor.route("surveillance"))
	go consumeIDEPStream(brokers, *kafkaGroupID, *kafkaIDEPTopic, a.DB, monitor.route("idep"))
	go consumeFlightStream(brokers, *kafkaGroupID, *kafkaFlightTopic, a.DB, monitor.route("flight"))

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

func consumeSurveillanceStream(brokers []string, groupID string, topic string, db *sql.DB, monitor *gatewayRouteMonitor) {
	runKafkaConsumerLoop(brokers, groupID, topic, monitor, func(value []byte) {
		monitor.RecordReceived()
		survController := controller.NewSurveillanceController(db)
		if onSurveillanceReceive(value, survController) {
			monitor.RecordProcessed()
		} else {
			monitor.RecordSkipped()
		}
	})
}

func consumeIDEPStream(brokers []string, groupID string, topic string, db *sql.DB, monitor *gatewayRouteMonitor) {
	flightController := controller.NewFlightController(db)
	runKafkaConsumerLoop(brokers, groupID, topic, monitor, func(value []byte) {
		monitor.RecordReceived()
		if onIDEPReceive(value, db, flightController) {
			monitor.RecordProcessed()
		} else {
			monitor.RecordSkipped()
		}
	})
}

func consumeFlightStream(brokers []string, groupID string, topic string, db *sql.DB, monitor *gatewayRouteMonitor) {
	flightController := controller.NewFlightController(db)
	runKafkaConsumerLoop(brokers, groupID, topic, monitor, func(value []byte) {
		monitor.RecordReceived()
		records, err := splitFlightPayloadRecords(value)
		if err != nil {
			monitor.RecordDecodeError(err)
			log.Println("error decoding flight payload:", err)
			return
		}

		for _, record := range records {
			command, err := extractFlightCommand(record)
			if err != nil {
				monitor.RecordDecodeError(err)
				log.Println("error decoding flight command:", err)
				continue
			}

			if isFlightPlanCommand(command) {
				if onFPLReceive(record, db, flightController) {
					monitor.RecordProcessed()
				} else {
					monitor.RecordSkipped()
				}
				continue
			}

			if !isNonFlightPlanCommand(command) {
				monitor.RecordSkipped()
				log.Println("ignored flight command:", command)
				continue
			}

			switch command {
			case "DEP", "ARR":
				if onCMDReceive(record, db, flightController) {
					monitor.RecordProcessed()
				} else {
					monitor.RecordSkipped()
				}
			case "CNL":
				onCNLReceive(record, db, flightController)
				monitor.RecordSkipped()
			case "CHG":
				onCHGReceive(record, db, flightController)
				monitor.RecordSkipped()
			case "DLA", "DLY":
				onDLYReceive(record, db, flightController)
				monitor.RecordSkipped()
			}
		}
	})
}

func runKafkaConsumerLoop(
	brokers []string,
	groupID string,
	topic string,
	monitor *gatewayRouteMonitor,
	handle func([]byte),
) {
	for {
		err := consumeTopic(brokers, groupID, topic, handle)
		monitor.RecordConsumerError(err)
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

func splitFlightPayloadRecords(payload []byte) ([][]byte, error) {
	trimmedPayload := bytes.TrimSpace(payload)
	if len(trimmedPayload) == 0 {
		return nil, errEmptyFlightPayload
	}

	if trimmedPayload[0] == '[' {
		var records []json.RawMessage
		if err := json.Unmarshal(trimmedPayload, &records); err != nil {
			return nil, err
		}

		result := make([][]byte, 0, len(records))
		for _, record := range records {
			trimmedRecord := bytes.TrimSpace(record)
			if len(trimmedRecord) == 0 || bytes.Equal(trimmedRecord, []byte("null")) {
				continue
			}
			result = append(result, append([]byte(nil), trimmedRecord...))
		}

		if len(result) == 0 {
			return nil, errEmptyFlightPayload
		}
		return result, nil
	}

	return [][]byte{append([]byte(nil), trimmedPayload...)}, nil
}

func extractFlightCommand(payload []byte) (string, error) {
	var data struct {
		CMD string `json:"CMD"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return "", err
	}

	command := normalizeFlightCommand(data.CMD)
	if command == "" {
		return "", errors.New("missing CMD field")
	}

	return command, nil
}

func normalizeFlightCommand(command string) string {
	return strings.ToUpper(strings.TrimSpace(command))
}

func isFlightPlanCommand(command string) bool {
	return normalizeFlightCommand(command) == "FPL"
}

func isNonFlightPlanCommand(command string) bool {
	switch normalizeFlightCommand(command) {
	case "ARR", "CNL", "CHG", "DLA", "DLY", "DEP":
		return true
	default:
		return false
	}
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
