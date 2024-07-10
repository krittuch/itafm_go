package app

import (
	"os"
)

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
)
