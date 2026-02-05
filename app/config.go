package app

import (
	// "log"
	"os"
	// "github.com/joho/godotenv"
)

// func goDotEnvVariable(key string) string {

// 	// load .env file
// 	err := godotenv.Load(".env")

// 	if err != nil {
// 		log.Fatalf("Error loading .env file")
// 	}

// 	return os.Getenv(key)
// }

// var (
// 	UNAMEDB = goDotEnvVariable("DB_USER")
// 	PASSDB  = goDotEnvVariable("DB_PASSWORD")
// 	HOSTDB  = goDotEnvVariable("DB_HOST")
// 	DBNAME  = goDotEnvVariable("DB_NAME")
// 	DBPORT  = goDotEnvVariable("DB_PORT")

// 	MQTT_IP_ADDRESS = goDotEnvVariable("MQTT_IP_ADDRESS")
// 	MQTT_PORT       = goDotEnvVariable("MQTT_PORT")
// 	MQTT_USER       = goDotEnvVariable("MQTT_USER")
// 	MQTT_PASSWORD   = goDotEnvVariable("MQTT_PASSWORD")

// 	MQTT_FLIGHT_MOVEMENT_TOPIC = goDotEnvVariable("MQTT_FLIGHT_MOVEMENT_TOPIC")
// 	MQTT_FLIGHT_MOVEMENT_QUEUE = goDotEnvVariable("MQTT_FLIGHT_MOVEMENT_QUEUE")
// 	MQTT_IDEP_TOPIC            = goDotEnvVariable("MQTT_IDEP_TOPIC")
// 	MQTT_SURV_TOPIC            = goDotEnvVariable("MQTT_SURV_TOPIC")

// 	ITAFM_MQTT_IP_ADDRESS = goDotEnvVariable("ITAFM_MQTT_IP_ADDRESS")
// 	ITAFM_MQTT_PORT       = goDotEnvVariable("ITAFM_MQTT_PORT")
// 	ITAFM_MQTT_USER       = goDotEnvVariable("ITAFM_MQTT_USER")
// 	ITAFM_MQTT_PASSWORD   = goDotEnvVariable("ITAFM_MQTT_PASSWORD")

// 	ITAFM_SURV_TOPIC = goDotEnvVariable("ITAFM_SURV_TOPIC")
// 	ITAFM_FLTH_TOPIC = goDotEnvVariable("ITAFM_FLTH_TOPIC")
// 	)

var (
	UNAMEDB = os.Getenv("DB_USER")
	PASSDB  = os.Getenv("DB_PASSWORD")
	HOSTDB  = os.Getenv("DB_HOST")
	DBNAME  = os.Getenv("DB_NAME")
	DBPORT  = os.Getenv("DB_PORT")

	ITAFM_MQTT_IP_ADDRESS = os.Getenv("ITAFM_MQTT_IP_ADDRESS")
	ITAFM_MQTT_PORT       = os.Getenv("ITAFM_MQTT_PORT")
	ITAFM_MQTT_USER       = os.Getenv("ITAFM_MQTT_USER")
	ITAFM_MQTT_PASSWORD   = lookupEnvWithFallback("ITAFM_MQTT_PASSWORD", "ITAFM_MQTT_PASS")

	ITAFM_SURV_TOPIC = os.Getenv("ITAFM_SURV_TOPIC")
	ITAFM_FLTH_TOPIC = os.Getenv("ITAFM_FLTH_TOPIC")

	KAFKA_BROKERS      = lookupEnvWithDefault("KAFKA_BROKERS", "localhost:9092")
	KAFKA_GROUP_ID     = lookupEnvWithDefault("KAFKA_GROUP_ID", "itafm-go-gateway")
	KAFKA_FLIGHT_TOPIC = lookupEnvWithDefault("KAFKA_FLIGHT_TOPIC", "itafm.flight_movement")
	KAFKA_IDEP_TOPIC   = lookupEnvWithDefault("KAFKA_IDEP_TOPIC", "itafm.idep")
	KAFKA_SURV_TOPIC   = lookupEnvWithDefault("KAFKA_SURV_TOPIC", "itafm.surveillance")
)

func lookupEnvWithDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func lookupEnvWithFallback(primary string, secondary string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	return os.Getenv(secondary)
}
