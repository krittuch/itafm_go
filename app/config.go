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
	MQTT_IDEP_TOPIC = os.Getenv("MQTT_IDEP_TOPIC")
)
