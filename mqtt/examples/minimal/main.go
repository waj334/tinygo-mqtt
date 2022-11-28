package main

import (
	"context"
	"github.com/waj334/tinygo-mqtt/mqtt"
	"github.com/waj334/tinygo-mqtt/mqtt/packets"
	"log"
	"net"
	"time"
)

func main() {
	// Open connection to test server
	conn, err := net.Dial("tcp", "test.mosquitto.org:1884")
	if err != nil {
		log.Fatalln(err)
	}

	// Create a new client
	client := mqtt.NewClient(conn)

	// Set up a connection packet
	connPacket := packets.Connect{
		Version:                    packets.MQTT5,
		ClientId:                   "super-secret-test-client3",
		Username:                   "not-used",
		Password:                   "supersecurepassword",
		WillRetain:                 false,
		WillQos:                    0,
		Will:                       nil,
		CleanSession:               true,
		KeepAlive:                  30,
		SessionExpiryInterval:      0,
		ReceiveMaximum:             0,
		MaximumPacketSize:          0,
		TopicAliasMaximum:          0,
		RequestResponseInformation: 0,
		RequestProblemInformation:  0,
		UserProperties:             nil,
	}

	// Attempt to connect
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	if err = client.Connect(ctx, connPacket); err != nil {
		log.Fatalln(err)
	}

	println("MQTT client connected!")

	// Start keep alive
	ticker := time.NewTicker(client.KeepAliveInterval())
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := client.KeepAlive(); err != nil {
					log.Fatalln(err)
				}
				println("ping")
			}
		}
	}()

	// Loop forever
	select {}
}
