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
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/waj334/tinygo-mqtt/mqtt"
	"github.com/waj334/tinygo-mqtt/mqtt/packets"
	"github.com/waj334/tinygo-mqtt/mqtt/packets/primitives"
	"github.com/waj334/tinygo-mqtt/mqtt/storage/memory"
)

func main() {
	// Call rand.Seed to initialize the client's default random number generator
	rand.Seed(time.Now().UnixNano())

	// Generate random client id
	clientId := fmt.Sprintf("super-secret-test-client-%d", rand.Int63())

	// Set up a connection packet
	connectPacket := &packets.Connect{
		Version:                    packets.MQTT5,
		ClientId:                   primitives.PrimitiveString(clientId),
		Username:                   "not-used",
		Password:                   "supersecurepassword",
		WillRetain:                 false,
		WillQos:                    0,
		Will:                       "",
		CleanSession:               false,
		KeepAlive:                  30,
		SessionExpiryInterval:      primitives.PrimitiveUint32((time.Minute * 5).Minutes()),
		ReceiveMaximum:             5,
		MaximumPacketSize:          0,
		TopicAliasMaximum:          0,
		RequestResponseInformation: 0,
		RequestProblemInformation:  0,
		UserProperties:             nil,
	}

	// Persist storage between connections
	storage := memory.NewStorage()

restart:

	// Open connection to test server
	// Note: For baremetal targets, replace the following with the necessary method of acquiring a Conn.
	//conn, err := net.Dial("tcp", "broker.hivemq.com:1883")
	conn, err := net.Dial("tcp", "test.mosquitto.org:1883")
	if err != nil {
		log.Fatalln(err)
	}

	// Create a new client
	client := mqtt.NewClient(conn)
	client.SetStorage(storage)

	// Create an event channel to be notified on by the client. This channel will hold at most 10 pending events.
	events := client.CreateEventChannel(10)

	// Set a 30-second deadline for connecting
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Attempt to connect
	if err = client.Connect(ctx, connectPacket); err != nil {
		log.Fatalln(err)
	}

	donePolling := make(chan struct{}, 1)

	// Begin polling
	go func(done <-chan struct{}) {
		// Use ticker to send periodic keep-alive control packets
		ticker := time.NewTicker(client.KeepAliveInterval())

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := client.KeepAlive(); err != nil {
					log.Println("Keep Alive Error:", err)
					return
				}
			default:
				// Set a deadline of 1 second for polling for incoming messages
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)

				// Poll for incoming messages
				if err := client.Poll(ctx); err != nil {
					cancel()
					log.Printf("Poll error: %v\n", err)
					return
				}
				cancel()
			}
		}
	}(donePolling)

	// Create an additional event channel receive only the events for the subscription
	topicEvents := client.CreateEventChannel(10)

	// Start event processing
	go func() {
		// Periodically send a publish to be consumed
		publishTicker := time.NewTicker(time.Second)
		//publishTicker.Stop()

		for {
			select {
			case <-publishTicker.C:
				// Publish a random number of messages all at once
				for i := 0; i < rand.Intn(15); i++ {
					if err = client.Publish(context.Background(), packets.Publish{
						Retain:  false,
						QoS:     packets.QoS0,
						Topic:   "/test/ping",
						Payload: []byte("pong"),
					}); err != nil {
						log.Println("Publish error:", err)
					}
				}
			case <-events.Done:
				log.Println("General event channel closed")
				return
			case e := <-events.C:
				if e != nil {
					switch e.PacketType {
					case packets.CONNACK:
						connack := e.Data.(*packets.Connack)
						log.Println("MQTT client connected!")

						if connack.SessionPresent {
							log.Println("The last session has been persisted by the server")
						} else {
							log.Println("The last session has not been persisted by the server")
							log.Println("Reason:", mqtt.ReasonCode(connack.ReasonCode))
						}

					case packets.DISCONNECT:
						disconnect := e.Data.(*packets.Disconnect)
						log.Printf("MQTT client has been disconnected: %2x %v", disconnect.ReasonCode, mqtt.ReasonCode(disconnect.ReasonCode))
					case packets.SUBACK:
						log.Println("Subscribed to topic(s)")
						//publishTicker.Reset(time.Second)
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
			}
		}
	}()

	// Set a 30-second deadline for subscribing to topics
	ctx, cancel = context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	topic := mqtt.Topic{}
	topic.SetFilter("/test/ping").
		SetQoS(packets.QoS0)
	topic.SetEventChannel(topicEvents)

	// Subscribe to topics
	if err = client.Subscribe(ctx, []mqtt.Topic{
		topic,
	}); err != nil {
		log.Fatalln("Subscribe error:", err)
	}

	// Unsubscribe after 30 seconds
	//time.AfterFunc(time.Second*30, func() {
	//	println("Unsubscribing from /test/ping")
	//	err := client.Unsubscribe(context.Background(), []string{
	//		"/test/ping",
	//	})
	//	if err != nil {
	//		log.Fatalln("Unsubscribe error:", err)
	//	}
	//})

	// QoS test: Close the conn to simulate an abrupt disconnect and restart.
	timer := time.NewTimer(time.Second * 30)
	select {
	case <-timer.C:
		// Close the conn and goto to the restart label
		donePolling <- struct{}{}
		conn.Close()

		// Close the event channel
		client.CloseEventChannel(events)
		client.CloseEventChannel(topicEvents)

		goto restart
	}

	select {}
}
