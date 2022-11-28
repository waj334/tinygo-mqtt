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
	"fmt"
	"github.com/waj334/tinygo-mqtt/mqtt"
	"github.com/waj334/tinygo-mqtt/mqtt/packets"
	"github.com/waj334/tinygo-mqtt/mqtt/packets/primitives"
	"log"
	"math/rand"
	"net"
	"time"
)

func main() {
	// Open connection to test server
	// Note: For baremetal targets, replace the following with the necessary method of acquiring a Conn.
	conn, err := net.Dial("tcp", "test.mosquitto.org:1883")
	if err != nil {
		log.Fatalln(err)
	}

	// Create a new client
	client := mqtt.NewClient(conn)

	// Create an event channel to be notified on by the client. This channel will hold at most 10 pending events.
	events := client.CreateEventChannel(10)

	// Generate random client id
	clientId := fmt.Sprintf("super-secret-test-client-%d", rand.Int63())

	// Set up a connection packet
	connPacket := packets.Connect{
		Version:                    packets.MQTT5,
		ClientId:                   primitives.PrimitiveString(clientId),
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

	// Set a 30-second deadline for connecting
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Attempt to connect
	if err = client.Connect(ctx, connPacket); err != nil {
		log.Fatalln(err)
	}

	// Create an additional event channel receive only the events for the subscription
	topicEvents := client.CreateEventChannel(10)

	// Start event processing
	go func() {
		// Use ticker to send periodic keep-alive control packets
		ticker := time.NewTicker(client.KeepAliveInterval())
		ticker2 := time.NewTicker(time.Second)

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
				if e != nil {
					switch e.PacketType {
					case packets.CONNACK:
						log.Println("MQTT client connected!")
					case packets.DISCONNECT:
						log.Println("MQTT client has been disconnected")
					case packets.SUBACK:
						log.Println("Subscribed to topic(s)")
					case packets.PUBLISH:
						pub := e.Data.(*packets.Publish)
						log.Println("General channel received publish:", string(pub.Payload))
						log.Println("Publish topic:", pub.Topic)
					default:
						println("Received packet:", e.PacketType)
					}
				}
			case <-topicEvents.Done:
				log.Println("Topic channel has been closed")
				log.Println("Client will remain connected")
				topicEvents.C = nil // Make sure this channel is never selected again
				topicEvents.Done = nil
			case e := <-topicEvents.C:
				if e != nil {
					switch e.PacketType {
					case packets.PUBLISH:
						pub := e.Data.(*packets.Publish)
						log.Println("Topic channel received publish:", string(pub.Payload))
						log.Println("Publish topic:", pub.Topic)
					}
				}
			default:
				// Set a deadline of 1 second for polling for incoming messages
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)

				// Poll for incoming messages
				if err := client.Poll(ctx); err != nil {
					cancel()
					log.Fatalf("Poll error: %v\n", err)
				}
				cancel()
			}
		}
	}()

	// Set a 30-second deadline for subscribing to topics
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	topic := mqtt.Topic{}
	topic.SetFilter("/test/ping").SetQoS(packets.QoS0)
	topic.SetEventChannel(topicEvents)

	// Subscribe to topics
	if err = client.Subscribe(ctx, []mqtt.Topic{
		topic,
	}); err != nil {
		log.Fatalln("Subscribe error:", err)
	}

	// Unsubscribe after 5 seconds
	time.AfterFunc(time.Second*5, func() {
		println("Unsubscribing from /test/ping")
		err := client.Unsubscribe(context.Background(), []string{
			"/test/ping",
		})
		if err != nil {
			log.Fatalln("Unsubscribe error:", err)
		}
	})

	// Loop forever
	select {}
}
