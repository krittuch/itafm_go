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

	MQTT_IP_ADDRESS = os.Getenv("MQTT_IP_ADDRESS")
	MQTT_PORT       = os.Getenv("MQTT_PORT")
	MQTT_USER       = os.Getenv("MQTT_USER")
	MQTT_PASSWORD   = os.Getenv("MQTT_PASSWORD")

	MQTT_FLIGHT_MOVEMENT_TOPIC = os.Getenv("MQTT_FLIGHT_MOVEMENT_TOPIC")
	MQTT_FLIGHT_MOVEMENT_QUEUE = os.Getenv("MQTT_FLIGHT_MOVEMENT_QUEUE")
	MQTT_IDEP_TOPIC            = os.Getenv("MQTT_IDEP_TOPIC")
	MQTT_SURV_TOPIC            = os.Getenv("MQTT_SURV_TOPIC")

	ITAFM_MQTT_IP_ADDRESS = os.Getenv("ITAFM_MQTT_IP_ADDRESS")
	ITAFM_MQTT_PORT       = os.Getenv("ITAFM_MQTT_PORT")
	ITAFM_MQTT_USER       = os.Getenv("ITAFM_MQTT_USER")
	ITAFM_MQTT_PASSWORD   = os.Getenv("ITAFM_MQTT_PASSWORD")

	ITAFM_SURV_TOPIC = os.Getenv("ITAFM_SURV_TOPIC")
	ITAFM_FLTH_TOPIC = os.Getenv("ITAFM_FLTH_TOPIC")
)
