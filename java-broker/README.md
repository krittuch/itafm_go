# ActiveMQ -> Kafka Broker

This broker consumes from ActiveMQ destinations and forwards raw payloads to Kafka topics.

## Environment variables

- `ACTIVEMQ_BROKER_URL` (optional, default uses `MQTT_IP_ADDRESS` + `MQTT_PORT`)
- `MQTT_USER`
- `MQTT_PASSWORD`
- `MQTT_FLIGHT_MOVEMENT_QUEUE`
- `MQTT_IDEP_QUEUE`
- `MQTT_SURV_TOPIC`
- `KAFKA_BROKERS`
- `KAFKA_FLIGHT_TOPIC`
- `KAFKA_IDEP_TOPIC`
- `KAFKA_SURV_TOPIC`
- `KAFKA_CLIENT_ID` (optional)

## Notes

- Destination prefixes such as `/queue/` and `/topic/` are supported.
- Payloads are forwarded without transformation.
- The process uses client acknowledge mode to avoid acknowledging ActiveMQ messages before Kafka accepts them.
