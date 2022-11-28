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
	conn, err := net.Dial("tcp", "test.mosquitto.org:1884")
	if err != nil {
		log.Fatalln(err)
	}

	// Create a new client
	client := mqtt.NewClient(conn)

	// Create an event channel to be notified on by the client
	events := client.CreateEventChannel()

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

	// Use ticker to send periodic keep-alive control packets
	ticker := time.NewTicker(client.KeepAliveInterval())

	// Start event processing loop
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := client.KeepAlive(); err != nil {
					log.Fatalln(err)
				}
				println("ping")
			case e := <-events.C:
				if e.PacketType == packets.CONNACK {
					println("MQTT client connected!")
				}
			}
		}
	}()

	// Loop forever
	select {}
}
