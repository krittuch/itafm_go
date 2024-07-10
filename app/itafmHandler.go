package app

import (
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	log.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	log.Printf("Connect lost: %v", err)
}

func initITAFM() mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%s", ITAFM_MQTT_IP_ADDRESS, ITAFM_MQTT_PORT))
	opts.SetClientID("itafm_aods_gw")
	opts.SetUsername(ITAFM_MQTT_USER)
	opts.SetPassword(ITAFM_MQTT_PASSWORD)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)

	return client
}

func sendToITAFM(client mqtt.Client, topic string, text string) {
	token := client.Publish(topic, 0, false, text)

	if token.Wait() && token.Error() != nil {
		log.Println(token.Error())
	}
}
