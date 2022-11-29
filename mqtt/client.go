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
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/waj334/tinygo-mqtt/mqtt/packets"
	"github.com/waj334/tinygo-mqtt/mqtt/packets/primitives"
	"github.com/waj334/tinygo-mqtt/mqtt/storage"
)

type Client struct {
	conn      net.Conn
	connMutex sync.Mutex
	mutex     sync.RWMutex

	storage storage.Storage

	isConnected           bool
	keepAliveInterval     time.Duration
	pingRespDeadline      time.Time
	sessionExpiryInterval uint32

	clientReceiveMaximum uint16
	serverReceiveMaximum uint16

	eventChans map[int]EventChannel
	topicChans map[string]EventChannel

	responseChan map[int]chan any

	evChanIdCounter int
	eventMutex      sync.Mutex

	sendQuota    uint16
	receiveQuota uint16

	rngFn func() uint32

	pendingSendSemaphore chan struct{}
}

type Topic struct {
	packets.Topic
	channel EventChannel
}

func (t *Topic) SetEventChannel(channel EventChannel) *Topic {
	t.channel = channel
	return t
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:            conn,
		eventChans:      make(map[int]EventChannel),
		topicChans:      make(map[string]EventChannel),
		responseChan:    make(map[int]chan any),
		evChanIdCounter: 1,
		rngFn:           rand.Uint32,
	}
}

// SetStorage sets the storage implementation that will be used to support the control packet persistence required for
// QoS 1 and QoS 2. No persistence will take place if no storage implementation is set which cause message delivery
// retry to be effectively disabled. No storage implementation is set by default.
func (c *Client) SetStorage(storage storage.Storage) {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	c.storage = storage
}

// SetRngFn set the random number generator function that will be used to generate random packet identifiers. The
// default is rand.Uint32.
func (c *Client) SetRngFn(fn func() uint32) {
	c.rngFn = fn
}

// CreateEventChannel creates an event channel struct that the client will use to notify when events (connect,
// disconnect, publish, subscribe, etc...) occur. /Consumers must consume a pending event before any incoming events can
// be received./ Prior events will not be signalled on the new channel.
func (c *Client) CreateEventChannel(n int) EventChannel {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	if n <= 0 {
		n = 1
	}

	// Create a channel for consumers to be signalled on
	channel := make(chan *Event, n)
	done := make(chan struct{}, 1)

	// Create the struct that will be returned to the caller
	result := EventChannel{
		C:    channel,
		Done: done,
		id:   c.evChanIdCounter,

		channel: channel,
		done:    done,
	}

	// Track this chan so fanout signalling can occur later
	c.eventChans[c.evChanIdCounter] = result
	c.evChanIdCounter++

	return result
}

// CloseEventChannel closes the event channel. No further events will be signalled on the channel.
func (c *Client) CloseEventChannel(channel EventChannel) {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	c.closeEventChannelInternal(channel)
}

func (c *Client) closeEventChannelInternal(channel EventChannel) {
	// Close the channel so that no further signals can occur on it
	close(channel.channel)

	// Send the done signal and close the done channel
	channel.done <- struct{}{}
	close(channel.done)

	// Remove channel from maps
	delete(c.eventChans, channel.id)
	for key, topicChan := range c.topicChans {
		if topicChan.id == channel.id {
			delete(c.topicChans, key)
		}
	}
}

// signal signals on all event channels in a fanout fashion. This function is only meant to be called by the client
// internally.
func (c *Client) signal(packetType packets.PacketType, data any, channel chan<- *Event) {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	e := &Event{
		PacketType: packetType,
		Data:       data,
	}

	if channel != nil {
		// Signal this channel directly
		select {
		case channel <- e:
			// Signalled
		default:
			// TODO: Decide whether or not to let this goroutine block. If this goroutine is allowed to block, then it
			//       will be required that no event channel goes unconsumed. Otherwise, the tradeoff would be unconsumed
			//       event channels will stop receiving new events when they are full.
			// Already has a pending event. This channel will miss the current event
		}
	} else {
		// Fanout to all other channels
		for _, channel := range c.eventChans {
			select {
			case channel.channel <- e:
				// Signalled
			default:
				// TODO: Decide whether or not to let this goroutine block. If this goroutine is allowed to block, then it
				//       will be required that no event channel goes unconsumed. Otherwise, the tradeoff would be unconsumed
				//       event channels will stop receiving new events when they are full.
				// Already has a pending event. This channel will miss the current event
			}
		}
	}

	// Sleep this goroutine to allow other goroutines to consume their event channels
	time.Sleep(time.Nanosecond)
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

	var unlockConn sync.Once
	c.connMutex.Lock()
	defer unlockConn.Do(c.connMutex.Unlock)

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
	unlockConn.Do(c.connMutex.Unlock)

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

	// Store receive maximum reported by the connect packet and CONNACK received from the server
	c.serverReceiveMaximum = connack.ReceiveMaximum.Value()

	if packet.ReceiveMaximum == 0 {
		// Default to that of the server
		c.clientReceiveMaximum = c.serverReceiveMaximum
	} else {
		// Use the value from the connect control packet
		c.clientReceiveMaximum = packet.ReceiveMaximum.Value()
	}

	// Initialize quotas
	c.sendQuota = c.serverReceiveMaximum
	c.receiveQuota = c.clientReceiveMaximum

	c.pendingSendSemaphore = make(chan struct{}) //, c.sendQuota)

	// Set the ping response deadline
	c.pingRespDeadline = time.Now().Add(c.keepAliveInterval * 2)

	// Need to keep what the value of session expiry interval was in order to ensure the DISCONNECT control packet is
	// set up correctly later.
	// SPEC: If the Session Expiry Interval in the CONNECT packet was zero, then it is a Protocol Error to set a
	//       non-zero Session Expiry Interval in the DISCONNECT packet sent by the Client.
	c.sessionExpiryInterval = uint32(packet.SessionExpiryInterval)

	// Successful connection!
	c.isConnected = true

	// Signal CONNACK event
	c.signal(packets.CONNACK, &connack, nil)

	return
}

// IsConnected returns true if the client is currently in the connected state. Otherwise, it returns false if the client
// is not currently connected to a MQTT server.
func (c *Client) IsConnected() bool {
	return c.isConnected
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

	if c.isConnected {
		return ErrClientNotConnected
	}

	var deadline time.Time
	var ok bool
	if deadline, ok = ctx.Deadline(); !ok {
		deadline = time.Time{}
	}

	var unlockConn sync.Once
	c.connMutex.Lock()
	defer unlockConn.Do(c.connMutex.Unlock)

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
		disconnect.SessionExpiryInterval = primitives.PrimitiveUint32(sessionExpiryInterval)
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
	unlockConn.Do(c.connMutex.Unlock)

	c.isConnected = false

	// Signal disconnect
	c.signal(packets.DISCONNECT, &disconnect, nil)

	return nil
}

func (c *Client) disconnectWithReason(ctx context.Context, reason primitives.PrimitiveByte) (err error) {
	if c.isConnected {
		return ErrClientNotConnected
	}

	var deadline time.Time
	var ok bool
	if deadline, ok = ctx.Deadline(); !ok {
		deadline = time.Time{}
	}

	var unlockConn sync.Once
	c.connMutex.Lock()
	defer unlockConn.Do(c.connMutex.Unlock)

	// Set I/O deadline
	if err = c.conn.SetDeadline(deadline); err != nil {
		return err
	}

	disconnect := packets.Disconnect{
		ReasonCode: reason,
	}

	// Only allow specifying the session expiry interval if it was set to a non-zero value in the CONNECT control
	// packet.
	// SPEC: If the Session Expiry Interval in the CONNECT packet was zero, then it is a Protocol Error to set a
	//       non-zero Session Expiry Interval in the DISCONNECT packet sent by the Client.
	//if c.sessionExpiryInterval != 0 {
	//	disconnect.SessionExpiryInterval = primitives.PrimitiveUint32(sessionExpiryInterval)
	//}

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
	unlockConn.Do(c.connMutex.Unlock)

	c.isConnected = false

	// Signal disconnect
	c.signal(packets.DISCONNECT, &disconnect, nil)

	return
}

// Subscribe sends the SUBSCRIBE control packet to the server with the specified topic filters and options.
func (c *Client) Subscribe(ctx context.Context, topics []Topic) (err error) {
	// Do nothing if topics list is empty
	if len(topics) == 0 {
		return ErrInvalidArgument
	}

	if !c.isConnected {
		return ErrClientNotConnected
	}

	var deadline time.Time
	var ok bool
	if deadline, ok = ctx.Deadline(); !ok {
		deadline = time.Time{}
	}

	var unlockConn sync.Once
	c.connMutex.Lock()
	defer unlockConn.Do(c.connMutex.Unlock)

	// Set I/O deadline
	if err = c.conn.SetDeadline(deadline); err != nil {
		return err
	}

	var _topics []packets.Topic
	for index := range topics {
		_topics = append(_topics, topics[index].Topic)
	}

	c.mutex.Lock()
	subscribe := packets.Subscribe{
		PacketIdentifier: primitives.PrimitiveUint16(c.rngFn()),
		Topics:           _topics,

		// TODO: Use context to set these optional parameters
		//SubscriptionIdentifier: 0,
		//UserProperties:         nil,
	}

	// Create channel to receive the response on
	respChan := make(chan any, 1)
	c.responseChan[int(subscribe.PacketIdentifier)] = respChan

	// Send the SUBSCRIBE control packet
	if _, err = subscribe.WriteTo(c.conn); err != nil {
		c.mutex.Unlock()
		return err
	}
	unlockConn.Do(c.connMutex.Unlock)
	c.mutex.Unlock()

	// Wait for the acknowledgement
	select {
	case resp := <-respChan:
		suback := resp.(*packets.Suback)

		// Check all reason codes
		for i, code := range suback.ReasonCodes {
			if code >= 0x80 {
				// TODO: Return all failure reason codes to caller somehow
				return ReasonCode(code)
			} else {
				// TODO: Consider session retention details here

				if chanid := topics[i].channel.id; chanid != 0 {
					// Map the event channel to the topic
					c.mutex.Lock()
					c.topicChans[topics[i].Topic.Filter()] = c.eventChans[chanid]

					// Remove this channel from the general event channel map
					delete(c.eventChans, chanid)
					c.mutex.Unlock()
				}
			}
		}
	}

	// Close the chan
	close(respChan)

	c.mutex.Lock()
	// Remove the channel from the map
	delete(c.responseChan, int(subscribe.PacketIdentifier))
	c.mutex.Unlock()

	return
}

// Unsubscribe sends the UNSUBSCRIBE control packet to the server with the specified topic filters. Any event channels
// bound to topics specified by the topics parameter will not receive any further publishes from said topics.
func (c *Client) Unsubscribe(ctx context.Context, topics []string) (err error) {
	// Do nothing if topics list is empty
	if len(topics) == 0 {
		return ErrInvalidArgument
	}

	c.mutex.Lock()

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

	var _topics []packets.Topic
	for index := range topics {
		t := packets.Topic{}
		t.SetFilter(topics[index])
		_topics = append(_topics, t)
	}
	unsubscribe := packets.Unsubscribe{
		PacketIdentifier: primitives.PrimitiveUint16(c.rngFn()),
		Topics:           _topics,

		// TODO: Use context to set these optional parameters
		//UserProperties:         nil,
	}

	// Create channel to receive the response on
	respChan := make(chan any, 1)
	c.responseChan[int(unsubscribe.PacketIdentifier)] = respChan

	// Send the UNSUBSCRIBE control packet
	if _, err = unsubscribe.WriteTo(c.conn); err != nil {
		return err
	}
	c.mutex.Unlock()

	// Wait for the acknowledgement
	select {
	case <-respChan:
		c.mutex.Lock()
		// Close any event channels bound to the topics
		for _, topic := range topics {
			if channel, ok := c.topicChans[topic]; ok {
				c.closeEventChannelInternal(channel)
			}
		}
		c.mutex.Unlock()
	}

	c.mutex.Lock()
	// Remove the channel from the map
	delete(c.responseChan, int(unsubscribe.PacketIdentifier))
	c.mutex.Unlock()

	return
}

// Publish sends PUBLISH control packet to the server.
func (c *Client) Publish(ctx context.Context, pub packets.Publish) (err error) {
	if !c.isConnected {
		return ErrClientNotConnected
	}

	var deadline time.Time
	var ok bool
	if deadline, ok = ctx.Deadline(); !ok {
		deadline = time.Time{}
	}

	var unlockConn sync.Once
	c.connMutex.Lock()
	defer unlockConn.Do(c.connMutex.Unlock)

	// Set I/O deadline
	if err = c.conn.SetDeadline(deadline); err != nil {
		return err
	}

	// Perform preflight packet persistence operations
	if pub.QoS > 0 {
		// Assign a packet identifier if none is set
		c.mutex.Lock()
		if pub.PacketIdentifier == 0 {
			pub.PacketIdentifier = primitives.PrimitiveUint16(c.rngFn())
		}

		if c.storage != nil {
			// Store this publish control packet
			if err = c.storage.Store(uint16(pub.PacketIdentifier), pub); err != nil {
				return err
			}
		}
		c.mutex.Unlock()
	}

	// SPEC: Each time the Client or Server sends a PUBLISH packet at QoS > 0, it decrements the send quota. If the send
	//       quota reaches zero, the Client or Server MUST NOT send any more PUBLISH packets with QoS > 0
	//       [MQTT-4.9.0-2].
	//
	//       It MAY continue to send PUBLISH packets with QoS 0, or it MAY choose to suspend sending these as well. The
	//       Client and Server MUST continue to process and respond to all other MQTT Control Packets even if the quota
	//       is zero [MQTT-4.9.0-3].
	if c.sendQuota == 0 && pub.QoS > 0 {
		// Delay sending this publish until one of the unacknowledged publishes is acknowledged
		c.connMutex.Unlock()
		c.pendingSendSemaphore <- struct{}{}
		c.connMutex.Lock()
	}
	// Write the publish
	if _, err = pub.WriteTo(c.conn); err != nil {

		return
	}
	unlockConn.Do(c.connMutex.Unlock)

	if pub.QoS > 0 {
		c.mutex.Lock()
		// Decrement the send quota counter
		c.sendQuota--
		c.mutex.Unlock()
	}

	return
}

// sendPuback will send the PUBACK control packet to the server. This API is only accessible via Publish when it is
// RECEIVED from the server during the Poll method.
func (c *Client) sendPuback(ctx context.Context, publish *packets.Publish) (err error) {
	// The packet identifier MUST be set
	if publish.PacketIdentifier == 0 {
		return packets.ErrControlPacketIsMalformed
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

	puback := &packets.Puback{
		PacketIdentifier: publish.PacketIdentifier,
	}

	c.mutex.RLock()
	// Send the PUBACK control packet to the server
	if _, err = puback.WriteTo(c.conn); err != nil {
		c.mutex.RUnlock()
		return
	}
	c.mutex.RUnlock()

	// Increment the receive quota counter so that more publishes (QoS > 0) can be received
	c.mutex.Lock()
	c.receiveQuota++
	c.mutex.Unlock()

	// No response to wait for

	return
}

// sendPubrec will send the PUBREC control packet to the server. This API is only accessible via Publish when it is
// RECEIVED from the server during the Poll method.
func (c *Client) sendPubrec(ctx context.Context, publish *packets.Publish) (err error) {
	// The packet identifier MUST be set
	if publish.PacketIdentifier == 0 {
		return packets.ErrControlPacketIsMalformed
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

	pubrec := &packets.Pubrec{
		Puback: packets.Puback{
			PacketIdentifier: publish.PacketIdentifier,
		},
	}

	// Send the PUBREC control packet to the server
	if _, err = pubrec.WriteTo(c.conn); err != nil {
		c.mutex.RUnlock()
		return
	}

	// No response to wait for

	return
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
	c.mutex.RLock()
	defer c.mutex.RUnlock()

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

	var unlockConn sync.Once
	c.connMutex.Lock()
	defer unlockConn.Do(c.connMutex.Unlock)

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
	unlockConn.Do(c.connMutex.Unlock)

	// NOTE: Poll handles receiving the response and disconnecting if no response has been sent within twice the keep
	// alive interval.
	return
}

// Poll polls for incoming control packets from the server. Incoming messages will be pushed to the back of the message
// queue and a single message at the front of the queue will be processed. This function should be called repeatedly. No
// call to the Publish method should take place on the same goroutine that a call to Poll takes place on as this could
// potentially cause a deadlock.
func (c *Client) Poll(ctx context.Context) (err error) {
	if !c.isConnected {
		return ErrClientNotConnected
	}

	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	// Check if current time is after the ping response deadline
	// SPEC: If a Client does not receive a PINGRESP packet within a reasonable amount of time after it has sent a
	//       PINGREQ, it SHOULD close the Network Connection to the Server.
	if time.Now().After(c.pingRespDeadline) {
		// Likely disconnected from server. Close the connection.
		// SPEC: [MQTT-3.1.2-22]
		c.isConnected = false
		if err := c.conn.Close(); err != nil {
			return err
		}

		// Signal synthetic DISCONNECT event
		// TODO: Determine if this is even necessary
		c.signal(packets.DISCONNECT, nil, nil)
		return
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

	// Set I/O deadline to 10ms initially so that polling doesn't tie up the conn for too long
	//if err = c.conn.SetDeadline(time.Now().Add(time.Millisecond * 10)); err != nil {
	//	return
	//}

	// Attempt to receive a control packet header
	header := packets.FixedHeader{}
	if _, err = header.ReadFrom(c.conn); errors.Is(err, os.ErrDeadlineExceeded) {
		// No incoming data
		return nil
	} else if err != nil {
		// Some other error occurred. Return it
		return
	}

	if !deadline.IsZero() {
		// Extend I/O deadline
		if err = c.conn.SetDeadline(time.Now().Add(time.Second * 30)); err != nil {
			return
		}
	} else {
		// Unset any deadline
		c.conn.SetDeadline(time.Time{})
	}

	// Read control packet
	switch header.GetType() {
	case packets.PUBLISH:
		publish := &packets.Publish{Header: header}
		if _, err = publish.ReadFrom(c.conn); err != nil {
			return
		}

		c.mutex.Lock()
		if c.receiveQuota == 0 && publish.QoS > 0 {
			// The server has sent more publishes than this client is willing to accept. Send disconnect.
			// SPEC: The Server MUST NOT send more than Receive Maximum QoS 1 and QoS 2 PUBLISH packets for which it has
			//       not received PUBACK, PUBCOMP, or PUBREC with a Reason Code of 128 or greater from the Client
			//       [MQTT-3.3.4-9]. If it receives more than Receive Maximum QoS 1 and QoS 2 PUBLISH packets where it
			//       has not sent a PUBACK or PUBCOMP in response, the Client uses DISCONNECT with Reason Code 0x93
			//       (Receive Maximum exceeded) as described in section 4.13 Handling errors.
			if err = c.disconnectWithReason(ctx, 0x93); err != nil {
				c.mutex.Unlock()
				return err
			}
		} else {
			// Decrement the receive quota counter
			c.receiveQuota--
		}
		c.mutex.Unlock()

		// Send the respective acknowledgement control packet type for the QoS level of the incoming publish.
		if publish.QoS == packets.QoS1 {
			if err = c.sendPuback(ctx, publish); err != nil {
				return err
			}
		} else if publish.QoS == packets.QoS2 {
			if err = c.sendPubrec(ctx, publish); err != nil {
				return err
			}
		}

		// Route the PUBLISH to the correct event channels as configured by the Subscribe API
		for filter, channel := range c.topicChans {
			// Does the topic match any known filter?
			if c.matchTopic(publish.Topic.String(), filter) {
				// Signal the publish on this channel
				c.signal(packets.PUBLISH, publish, channel.channel)
			}
		}

		// Create a Pubrec control packet and store it. This might be used later during the message delivery retry flow.
		// It will be removed when a PUBREL control packet comes in.
		c.mutex.RLock()
		if publish.QoS > 0 && c.storage != nil {
			pubrec := &packets.Pubrec{
				Puback: packets.Puback{
					PacketIdentifier: publish.PacketIdentifier,
				},
			}
			c.storage.Store(pubrec.PacketIdentifier.Value(), pubrec)
			c.mutex.RUnlock()
		}

		c.signal(packets.PUBLISH, publish, nil)
	case packets.PUBACK:
		puback := &packets.Puback{Header: header}
		if _, err = puback.ReadFrom(c.conn); err != nil {
			return err
		}

		// Perform rate limiting related operations
		c.mutex.Lock()
		if c.sendQuota < c.serverReceiveMaximum {
			// Increment the sent publish counter
			c.sendQuota++

			// Potentially allow a blocked call to Publish to continue.
			// NOTE: This is the reason why calls to Publish and Poll should not occur within the same goroutine.
			select {
			case <-c.pendingSendSemaphore: // Unblock a pending call to Publish
			default: // No call to Publish is pending
			}
		}

		// Drop any persisted publish with the same packet identifier
		if c.storage != nil {
			if err = c.storage.Drop(puback.PacketIdentifier.Value()); err != nil {
				return err
			}
		}
		c.mutex.Unlock()

		c.signal(packets.PUBACK, puback, nil)
	case packets.PUBREC:
		pubrec := &packets.Pubrec{}
		pubrec.Header = header
		if _, err = pubrec.ReadFrom(c.conn); err != nil {
			return err
		}

		// Increment the send quota counter if it contains a failure reason code
		// SPEC: Each time a PUBREC packet is received with a Return Code of 0x80 or greater.
		c.mutex.Lock()
		if pubrec.ReasonCode > 0x80 {
			if c.sendQuota < c.serverReceiveMaximum {
				c.sendQuota++
			}
		}

		if c.storage != nil {
			// Discard original publish from persistent storage
			if err = c.storage.Drop(pubrec.PacketIdentifier.Value()); err != nil {
				return err
			}

			// Store the incoming PUBREC to persistent storage
			if err = c.storage.Store(pubrec.PacketIdentifier.Value(), pubrec); err != nil {
				return err
			}
		}

		// Send PUBREL control packet
		pubrel := &packets.Pubrel{
			Puback: packets.Puback{
				PacketIdentifier: pubrec.PacketIdentifier,
			},
		}

		if _, err = pubrel.WriteTo(c.conn); err != nil {
			return err
		}

		c.mutex.Unlock()

		c.signal(packets.PUBREC, pubrec, nil)
	case packets.PUBREL:
		pubrel := &packets.Pubrel{}
		pubrel.Header = header
		if _, err = pubrel.ReadFrom(c.conn); err != nil {
			return err
		}

		// Perform persistence operations as required by the QoS level of the related PUBLISH.
		c.mutex.Lock()
		if c.storage != nil {
			// Discard original PUBREC control packet from persistent storage
			if err = c.storage.Drop(pubrel.PacketIdentifier.Value()); err != nil {
				return err
			}
		}
		c.mutex.Unlock()

		// Send PUBCOMP control packet
		pubcomp := &packets.Pubcomp{
			Puback: packets.Puback{
				PacketIdentifier: pubrel.PacketIdentifier,
			},
		}

		if _, err = pubcomp.WriteTo(c.conn); err != nil {
			return err
		}

		// Increment receive quota counter
		if c.receiveQuota < c.clientReceiveMaximum {
			c.receiveQuota++
		}

		c.signal(packets.PUBREL, pubrel, nil)
	case packets.PUBCOMP:
		pubcomp := &packets.Pubcomp{}
		pubcomp.Header = header
		if _, err = pubcomp.ReadFrom(c.conn); err != nil {
			return err
		}

		// Perform rate limiting related operations
		c.mutex.Lock()
		if c.sendQuota < c.serverReceiveMaximum {
			// Increment the sent publish counter
			c.sendQuota++

			// Potentially allow a blocked call to Publish to continue.
			// NOTE: This is the reason why calls to Publish and Poll should not occur within the same goroutine.
			select {
			case <-c.pendingSendSemaphore: // Unblock a pending call to Publish
			default: // No call to Publish is pending
			}
		}

		if c.storage != nil {
			// Discard original PUBREC control packet from persistent storage
			if err = c.storage.Drop(pubcomp.PacketIdentifier.Value()); err != nil {
				return err
			}
		}

		c.mutex.Unlock()

		c.signal(packets.PUBCOMP, pubcomp, nil)
	case packets.SUBACK:
		suback := &packets.Suback{Header: header}
		if _, err = suback.ReadFrom(c.conn); err != nil {
			return
		}

		// Respond to the call to client.Subscribe
		if respChan, ok := c.responseChan[int(suback.PacketIdentifier)]; ok {
			respChan <- suback
		}

		c.signal(packets.SUBACK, suback, nil)
	case packets.UNSUBACK:
		unsuback := &packets.Unsuback{Header: header}
		if _, err = unsuback.ReadFrom(c.conn); err != nil {
			return
		}

		// Respond to the call to client.Unsubscribe
		if respChan, ok := c.responseChan[int(unsuback.PacketIdentifier)]; ok {
			respChan <- unsuback
		}

		c.signal(packets.UNSUBACK, unsuback, nil)
	case packets.DISCONNECT:
		disconnect := &packets.Disconnect{Header: header}
		if _, err = disconnect.ReadFrom(c.conn); err != nil {
			return
		}
		// Close the connection
		if err = c.conn.Close(); err != nil {
			return
		}
		c.signal(packets.DISCONNECT, disconnect, nil)
	case packets.AUTH:
		auth := &packets.Auth{Header: header}
		if _, err = auth.ReadFrom(c.conn); err != nil {
			return
		}
		c.signal(packets.AUTH, auth, nil)
	case packets.PINGRESP:
		// Extend the ping response deadline
		c.pingRespDeadline = time.Now().Add(c.keepAliveInterval * 2)
	default:
		return ErrUnexpectedPacketTypeReceived
	}

	// TODO: Process control packet

	return nil
}

// matchTopic returns true if the input topic string matches the topic filter string. Otherwise, it returns false.
func (c *Client) matchTopic(topic, filter string) bool {
	// TODO: Support matching for shared topics
	var filterPos int
	var topicPos int
	for filterPos < len(filter) {
		if filter[filterPos] == '#' {
			// Encountered multi-level wildcard.

			// Quick path
			if len(filter) == 1 {
				return true
			}

			// Look around the wildcard
			if (filterPos != 0 && filter[filterPos-1] != '/') || filterPos != len(filter)-1 {
				// Invalid use of # wildcard. Do attempt to match the filter any further
				return false
			}

			// Stop and return true
			return true
		} else if filter[filterPos] == '+' {
			// Encountered single-level wildcard

			// Look around the wildcard
			if (filterPos != 0 && filter[filterPos-1] != '/') || (filterPos != len(filter)-1 && filter[filterPos+1] != '/') {
				// Invalid use of + wildcard. Do attempt to match the filter any further
				return false
			}

			// Fast-forward the topic position to the beginning of the next level
			for topicPos < len(topic) && topic[topicPos] != '/' {
				topicPos++
			}

			if topicPos == len(topic) {
				// No levels left. Return true.
				return true
			}

			// Advance the filter pos and continue at the beginning of the loop
			filterPos++
			continue
		} else if filterPos >= len(topic) {
			// The length of the filter exceeded the length of the topic. No way these can match.
			return false
		} else if filter[filterPos] != topic[topicPos] {
			return false
		}

		filterPos++
		topicPos++
	}

	// Check if there is more characters in the topic that went unprocessed
	if len(filter) != len(topic) && topicPos < len(topic) {
		// Topic couldn't have matched the filter
		return false
	}

	return true
}
