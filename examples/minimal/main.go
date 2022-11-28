/*
 * MIT License
 *
 * Copyright (c) 2022-2022 waj334
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

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
	conn, err := net.Dial("tcp", "test.mosquitto.org:1883")
	if err != nil {
		log.Fatalln(err)
	}

	// Create a new client
	client := mqtt.NewClient(conn)

	// Create an event channel to be notified on by the client
	events := client.CreateEventChannel(10)

	// Set up a connection packet
	connPacket := packets.Connect{
		Version:                    packets.MQTT5,
		ClientId:                   "super-secret-test-client",
		Username:                   "not-used",
		Password:                   "supersecurepassword",
		WillRetain:                 false,
		WillQos:                    0,
		Will:                       "",
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
	//ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	if err = client.Connect(context.Background(), connPacket); err != nil {
		//cancel()
		log.Fatalln(err)
	}
	//cancel()

	// Subscribe to topics
	//ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	topic := mqtt.Topic{}
	topic.SetFilter("/test/ping").SetQoS(packets.QoS0)

	if err = client.Subscribe(context.Background(), []mqtt.Topic{
		topic,
	}); err != nil {
		//cancel()
		log.Fatalln("Subscribe error:", err)
	}
	//cancel()

	// Use ticker to send periodic keep-alive control packets
	ticker := time.NewTicker(client.KeepAliveInterval())
	ticker2 := time.NewTicker(time.Second)

	// Start event processing loop
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := client.KeepAlive(); err != nil {
					log.Fatalln("Keep Alive Error:", err)
				}
			case <-ticker2.C:
				// Publish a message
				if err = client.Publish(context.Background(), packets.Publish{
					Retain:  false,
					QoS:     0,
					Topic:   "/test/ping",
					Payload: []byte("pong"),
				}); err != nil {
					log.Println("Publish error:", err)
				}
			case e := <-events.C:
				switch e.PacketType {
				case packets.CONNACK:
					log.Println("MQTT client connected!")
				case packets.DISCONNECT:
					log.Println("MQTT client has been disconnected")
				case packets.SUBACK:
					log.Println("Subscribed to topic(s)")
				case packets.PUBLISH:
					pub := e.Data.(*packets.Publish)
					log.Println("Received publish:", string(pub.Payload))
				default:
					println("Received packet:", e.PacketType)
				}
			default:
				// Poll for incoming messages
				if err := client.Poll(); err != nil {
					log.Fatalf("Poll error: %v\n", err)
				}
			}
		}
	}()

	// Loop forever
	select {}
}
