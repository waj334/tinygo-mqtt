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

package mqtt

import (
	"context"
	"errors"
	"github.com/waj334/tinygo-mqtt/mqtt/packets"
	"net"
	"os"
	"sync"
	"time"
)

type Client struct {
	conn  net.Conn
	mutex sync.Mutex

	isConnected           bool
	keepAliveInterval     time.Duration
	sessionExpiryInterval uint32

	eventChans      map[int]chan<- Event
	evChanIdCounter int
	eventMutex      sync.Mutex
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:       conn,
		eventChans: make(map[int]chan<- Event),
	}
}

// CreateEventChannel creates an event channel struct that the client will use to notify when events (connect,
// disconnect, publish, subscribe, etc...) occur. Consumers must consume a pending event before any incoming events can
// be received. Prior events will not be signalled on the new channel.
func (c *Client) CreateEventChannel() EventChannel {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	// Create a channel for consumers to be signalled on
	channel := make(chan Event, 1)

	// Create the struct that will be returned to the caller
	result := EventChannel{
		C:  channel,
		id: c.evChanIdCounter,
	}

	// Track this chan so fanout signalling can occur later
	c.eventChans[c.evChanIdCounter] = channel
	c.evChanIdCounter++

	return result
}

// CloseEventChannel closes the event channel. No further events will be signalled on the channel.
func (c *Client) CloseEventChannel(eventChan EventChannel) {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	if channel, ok := c.eventChans[eventChan.id]; ok {
		// Close the channel so that no further signals can occur on it
		close(channel)
	}
}

// signal signals on all event channels in a fanout fashion. This function is only meant to be called by the client
// internally.
func (c *Client) signal(packetType packets.PacketType, data any) {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	e := Event{
		PacketType: packetType,
		Data:       data,
	}

	// Fanout
	for _, channel := range c.eventChans {
		select {
		case channel <- e:
			// Signalled
		default:
			// Already has a pending event. This channel will miss the current event
		}
	}
}

// Connect sends the CONNECT packet to the server and waits for the server to send the acknowledgement (CONNACK) packet
// back to the client. If the acknowledgement contains a failure reason, then a ReasonCode error is returned.
func (c *Client) Connect(ctx context.Context, packet packets.Connect) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var deadline time.Time
	var ok bool
	if deadline, ok = ctx.Deadline(); !ok {
		deadline = time.Time{}
	}

	// Set I/O deadline
	if err = c.conn.SetDeadline(deadline); err != nil {
		return err
	}

	// Send connect packet
	if _, err = packet.WriteTo(c.conn); err != nil {
		return err
	}

	// Receive response header
	header := packets.FixedHeader{}
	if err = backoff(ctx, func() error {
		_, err := header.ReadFrom(c.conn)
		return err
	}); err != nil {
		return
	}

	// Response must be CONNACK
	if header.GetType() != packets.CONNACK {
		return ErrUnexpectedPacketTypeReceived
	}

	// Create the Connack packet
	connack := packets.Connack{
		Header: header,
	}
	// Begin reading the CONNACK response
	if err = backoff(ctx, func() error {
		_, err := connack.ReadFrom(c.conn)
		return err
	}); err != nil {
		return
	}

	// Did the server send an error response?
	// SPEC: If a Server sends a CONNACK packet containing a Reason code of 128 or greater it MUST then close the
	//       Network Connection [MQTT-3.2.2-7].
	if connack.ReasonCode >= 128 {
		// Close the connection
		if err = c.conn.Close(); err != nil {
			return
		}

		// Error the ACK as the error
		return ReasonCode(connack.ReasonCode)
	}

	// Handle server keep alive specification
	if connack.ServerKeepAlive > 0 {
		// Use the keep alive interval returned by the server
		// SPEC: If the Server returns a Server Keep Alive on the CONNACK packet, the Client MUST use that value instead
		//       of the value it sent as the Keep Alive. [MQTT-3.1.2-21]
		c.keepAliveInterval = time.Second * time.Duration(connack.ServerKeepAlive)
	} else if packet.KeepAlive > 0 {
		// Use the keep alive interval defined in the CONNECT packet
		c.keepAliveInterval = time.Second * time.Duration(packet.KeepAlive)
	}

	// Need to keep what the value of session expiry interval was in order to ensure the DISCONNECT control packet is
	// set up correctly later.
	// SPEC: If the Session Expiry Interval in the CONNECT packet was zero, then it is a Protocol Error to set a
	//       non-zero Session Expiry Interval in the DISCONNECT packet sent by the Client.
	c.sessionExpiryInterval = packet.SessionExpiryInterval

	// Successful connection!
	c.isConnected = true

	// Signal CONNACK event
	c.signal(packets.CONNACK, connack)

	return
}

// Disconnect sends the DISCONNECT packet to the server. The network connection will be closed upon sending the
// DISCONNECT packet. Setting the publishWill parameter to true will require the server to publish the "Will" message if
// one was specified initially in the CONNECT packet. The server will default session expiry interval to that of the
// CONNECT control packet.
func (c *Client) Disconnect(ctx context.Context, publishWill bool) (err error) {
	return c.DisconnectWithSessionExpiry(ctx, publishWill, 0)
}

// DisconnectWithSessionExpiry sends the DISCONNECT packet to the server. The network connection will be closed upon
// sending the DISCONNECT packet. Setting the publishWill parameter to true will require the server to publish the
// "Will" message if one was specified initially in the CONNECT packet. The sessionExpiryInterval specifies the duration
// that server should maintain its MQTT session state for this client after it disconnects for an extended period of
// time. Setting a zero value for the sessionExpiryInterval parameter will cause the server to default to the value
// specified in the CONNECT control packet.
func (c *Client) DisconnectWithSessionExpiry(ctx context.Context, publishWill bool, sessionExpiryInterval int) (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.isConnected {
		return ErrClientNotConnected
	}

	var deadline time.Time
	var ok bool
	if deadline, ok = ctx.Deadline(); !ok {
		deadline = time.Time{}
	}

	// Set I/O deadline
	if err = c.conn.SetDeadline(deadline); err != nil {
		return err
	}

	disconnect := packets.Disconnect{}

	if publishWill {
		// Set the reason code to 0x04
		// SPEC: The Client wishes to disconnect but requires that the Server also publishes its Will Message.
		disconnect.ReasonCode = 0x04
	}

	// Only allow specifying the session expiry interval if it was set to a non-zero value in the CONNECT control
	// packet.
	// SPEC: If the Session Expiry Interval in the CONNECT packet was zero, then it is a Protocol Error to set a
	//       non-zero Session Expiry Interval in the DISCONNECT packet sent by the Client.
	if c.sessionExpiryInterval != 0 {
		disconnect.SessionExpiryInterval = uint32(sessionExpiryInterval)
	}

	// Send the DISCONNECT packet to the server
	if _, err = disconnect.WriteTo(c.conn); err != nil {
		return
	}

	// Close the connection to the server
	// SPEC: MUST NOT send any more MQTT Control Packets on that Network Connection [MQTT-3.14.4-1].
	//       MUST close the Network Connection [MQTT-3.14.4-2].
	if err = c.conn.Close(); err != nil {
		return
	}

	c.isConnected = false

	// Signal disconnect
	c.signal(packets.DISCONNECT, disconnect)

	return nil
}

// KeepAliveInterval returns the interval at which frequent PINGREQ packets must be sent. The server may specify a
// different value in the CONNACK packet than what was originally specified by the CONNECT packet.
func (c *Client) KeepAliveInterval() time.Duration {
	return c.keepAliveInterval
}

// KeepAlive sends the PINGREQ control packet to the server and then waits for the PINGRESP packet to be received.
// An error will be returned if any control packet other than PINGRESP is received by the client or the transmission
// timed out.
// SPEC: If Keep Alive is non-zero and in the absence of sending any other MQTT Control Packets, the Client MUST send a
//
//	PINGREQ packet. [MQTT-3.1.2-20]
func (c *Client) KeepAlive() (err error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.isConnected {
		return ErrClientNotConnected
	}

	// Wait for the response
	// SPEC: If the Keep Alive value is non-zero and the Server does not receive an MQTT Control Packet from the Client
	//       within one and a half times the Keep Alive time period, it MUST close the Network Connection to the Client
	//       as if the network had failed. [MQTT-3.1.2-22]
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(c.keepAliveInterval.Seconds()*1.5))
	defer cancel()

	var deadline time.Time
	var ok bool
	if deadline, ok = ctx.Deadline(); !ok {
		deadline = time.Time{}
	}

	// Set I/O deadline
	if err = c.conn.SetDeadline(deadline); err != nil {
		return err
	}

	// Send the PINGREQ control packet
	header := packets.FixedHeader{}
	header.SetType(packets.PINGREQ)
	if _, err = header.WriteTo(c.conn); err != nil {
		return
	}

	if err = backoff(ctx, func() error {
		_, err := header.ReadFrom(c.conn)
		return err
	}); errors.Is(err, os.ErrDeadlineExceeded) {
		// Likely disconnected from server. Close the connection.
		// SPEC: [MQTT-3.1.2-22]
		c.isConnected = false
		if err := c.conn.Close(); err != nil {
			return err
		}

		// Signal synthetic DISCONNECT event
		// TODO: Determine if this is even necessary
		c.signal(packets.DISCONNECT, nil)

		return
	} else if err != nil {
		return
	}

	if header.GetType() != packets.PINGRESP {
		return ErrUnexpectedPacketTypeReceived
	}

	return
}
