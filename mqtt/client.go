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

	isConnected       bool
	keepAliveInterval time.Duration
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn: conn,
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
	ack := packets.Connack{
		Header: header,
	}
	// Begin reading the CONNACK response
	if err = backoff(ctx, func() error {
		_, err := ack.ReadFrom(c.conn)
		return err
	}); err != nil {
		return
	}

	// Did the server send an error response?
	// SPEC: If a Server sends a CONNACK packet containing a Reason code of 128 or greater it MUST then close the
	//       Network Connection [MQTT-3.2.2-7].
	if ack.ReasonCode >= 128 {
		// Close the connection
		if err = c.conn.Close(); err != nil {
			return
		}

		// Error the ACK as the error
		return ReasonCode(ack.ReasonCode)
	}

	// Handle server keep alive specification
	if ack.ServerKeepAlive > 0 {
		// Use the keep alive interval returned by the server
		// SPEC: If the Server returns a Server Keep Alive on the CONNACK packet, the Client MUST use that value instead
		//       of the value it sent as the Keep Alive. [MQTT-3.1.2-21]
		c.keepAliveInterval = time.Second * time.Duration(ack.ServerKeepAlive)
	} else if packet.KeepAlive > 0 {
		// Use the keep alive interval defined in the CONNECT packet
		c.keepAliveInterval = time.Second * time.Duration(packet.KeepAlive)
	}

	// Successful connection!
	c.isConnected = true

	// TODO: Execute connect hook
	return
}

// Disconnect sends the DISCONNECT packet to the server. The network connection will be closed upon sending the
// DISCONNECT packet. Setting the publishWill parameter to true will require the server to publish the "Will" message if
// one was specified initially in the CONNECT packet.
func (c *Client) Disconnect(ctx context.Context, publishWill bool) (err error) {
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

	// TODO: Execute disconnect hook

	return nil
}

// KeepAliveInterval returns the interval at which frequent PINGREQ packets must be sent. The server may specify a
// different value in the CONNACK packet than what was originally specified by the CONNECT packet.
func (c *Client) KeepAliveInterval() time.Duration {
	return c.keepAliveInterval
}

// KeepAlive sends the PINGREQ packet to the server and then waits for the PINGRESP packet to be received. An error will
// be returned if any packet other than PINGRESP is received by the client or the transmission timed out.
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

	// Send the PINGREQ packet
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
		c.isConnected = false
		if err := c.conn.Close(); err != nil {
			return err
		}

		// TODO: Execute disconnect hook

		return
	} else if err != nil {
		return
	}

	if header.GetType() != packets.PINGRESP {
		return ErrUnexpectedPacketTypeReceived
	}

	return
}
