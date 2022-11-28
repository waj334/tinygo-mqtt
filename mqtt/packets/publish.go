/*
 * MIT License
 *
 * Copyright (c) 2022 waj334
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

package packets

import (
	"context"
	"io"

	"github.com/waj334/tinygo-mqtt/mqtt/packets/primitives"
)

type Publish struct {
	Header    FixedHeader
	Retain    bool
	QoS       QoS
	Duplicate bool

	Topic            primitives.PrimitiveString
	PacketIdentifier primitives.PrimitiveUint16

	Payload []byte
	offset  int

	/* Properties */
	MessageExpiryInterval  primitives.PrimitiveUint32
	TopicAlias             primitives.PrimitiveUint16
	ResponseTopic          primitives.PrimitiveString
	CorrelationData        primitives.PrimitiveString
	UserProperties         primitives.PrimitiveStringMap
	SubscriptionIdentifier primitives.VariableByteInt
	ContentType            primitives.PrimitiveString

	/* Misc */
	ackFn func(context.Context, *Publish) error
}

func (p *Publish) Write(buf []byte) (n int, err error) {
	// Return EOF if the offset is at the end of the payload buffer
	if p.offset == len(p.Payload) {
		return 0, io.EOF
	}

	// Copy into payload
	n = copy(p.Payload[:p.offset], buf)
	p.offset += n

	return
}

func (p *Publish) Read(buf []byte) (n int, err error) {
	// Return EOF if the offset is at the end of the payload buffer
	if p.offset == len(p.Payload) {
		return 0, io.EOF
	}

	// Copy into buf
	n = copy(buf, p.Payload[p.offset:])
	p.offset += n

	return
}

func (p *Publish) Reset() {
	p.offset = 0
}

func (p *Publish) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	// Read the header from the reader if it has not been initialized
	if p.Header.GetType() == 0 {
		if count, err = p.Header.ReadFrom(r); err != nil {
			return
		}
		n += count
	}

	// Parse flags
	p.Retain = (p.Header.GetFlags() & 0x01) != 0
	p.QoS = QoS(p.Header.GetFlags()>>1) & 0x03
	p.Duplicate = ((p.Header.GetFlags() >> 3) & 0x01) != 0

	// Read variable header
	if count, err = p.Topic.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if count, err = p.PacketIdentifier.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(p.Header.Remaining) {
		return
	}

	/* Properties start */
	var propertiesLen primitives.VariableByteInt
	if count, err = propertiesLen.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(p.Header.Remaining) {
		return
	}

	remaining := int64(propertiesLen)
	for remaining > 0 {
		// Read the identifier byte
		var identifier byte
		if identifier, err = primitives.ReadByte(r); err != nil {
			return 0, err
		}
		n++
		remaining--

		switch identifier {
		case 0x02: // Message expiry interval
			if count, err = p.MessageExpiryInterval.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x23: // Topic alias
			if count, err = p.TopicAlias.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x08: // Response topic
			if count, err = p.ResponseTopic.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x09: // Correlation data
			if count, err = p.CorrelationData.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x26: // User Property
			if p.UserProperties == nil {
				p.UserProperties = make(primitives.PrimitiveStringMap)
			}
			var k, v primitives.PrimitiveString
			if count, err = k.ReadFrom(r); err != nil {
				return 0, err
			}

			var count2 int64
			if count2, err = v.ReadFrom(r); err != nil {
				return 0, err
			}
			count += count2
			p.UserProperties[k] = v
		case 0x0B: // Subscription indentifier
			if count, err = p.SubscriptionIdentifier.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x03: // Content type
			if count, err = p.ContentType.ReadFrom(r); err != nil {
				return 0, err
			}
		}
		n += count
		remaining -= count
	}
	/* Properties end */

	// Read the payload
	payloadLen := int64(p.Header.Remaining) - n
	if payloadLen > 0 {
		p.Payload = make([]byte, payloadLen)
		if count, err := primitives.Read(r, p.Payload); err != nil {
			return 0, err
		} else {
			n += int64(count)
		}
	}

	return
}

func (p *Publish) WriteTo(w io.Writer) (n int64, err error) {
	var flags primitives.PrimitiveByte
	variableHeaderLen := primitives.VariableByteInt(0)
	propertiesLen := primitives.VariableByteInt(0)
	payloadLen := primitives.VariableByteInt(len(p.Payload))

	// Fail early if the topic is zero-length and no topic alias is specified.
	// SPEC: The Topic Name MUST be present as the first field in the PUBLISH packet Variable Header. It MUST be a UTF-8
	//       Encoded String as defined in section 1.5.4 [MQTT-3.3.2-1].
	// SPEC: If a Topic Alias mapping has been set at the receiver, a sender can send a PUBLISH packet that contains
	//       that Topic Alias and a zero length Topic Name.
	if len(p.Topic) == 0 && p.TopicAlias == 0 {
		return 0, ErrControlPacketIsMalformed
	}

	// Calculate length of properties and payload
	variableHeaderLen += p.Topic.Length(false)
	variableHeaderLen += p.PacketIdentifier.Length(false)

	if p.MessageExpiryInterval > 0 {
		propertiesLen += p.MessageExpiryInterval.Length(true)
	}

	if p.TopicAlias > 0 {
		propertiesLen += p.TopicAlias.Length(true)
	}

	if len(p.ResponseTopic) > 0 {
		propertiesLen += p.ResponseTopic.Length(true)
	}

	if len(p.CorrelationData) > 0 {
		propertiesLen += p.CorrelationData.Length(true)
	}

	for k, v := range p.UserProperties {
		propertiesLen += 1 + k.Length(false) + v.Length(false)
	}

	if p.SubscriptionIdentifier > 0 {
		propertiesLen += p.SubscriptionIdentifier.Length(true)
	}

	if len(p.ContentType) > 0 {
		propertiesLen += p.ContentType.Length(true)
	}

	variableHeaderLen += propertiesLen.Length(false) + propertiesLen

	// Set flags
	if p.Retain {
		flags |= 1 << 0
	}

	flags |= primitives.PrimitiveByte(p.QoS) << 1

	if p.Duplicate {
		flags |= 1 << 3
	}

	p.Header.SetType(PUBLISH)
	p.Header.SetFlags(flags)
	p.Header.Remaining = variableHeaderLen + payloadLen

	var count int64

	// Write fixed header
	if n, err = p.Header.WriteTo(w); err != nil {
		return 0, err
	}

	if count, err = p.Topic.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	if count, err = p.PacketIdentifier.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	/* Properties start */
	if count, err = propertiesLen.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	if p.MessageExpiryInterval > 0 {
		if count, err = p.MessageExpiryInterval.WriteToAsProperty(0x02, w); err != nil {
			return 0, err
		}
		n += count
	}

	if p.TopicAlias > 0 {
		if count, err = p.TopicAlias.WriteToAsProperty(0x23, w); err != nil {
			return 0, err
		}
		n += count
	}

	if len(p.ResponseTopic) > 0 {
		if count, err = p.ResponseTopic.WriteToAsProperty(0x08, w); err != nil {
			return 0, err
		}
		n += count
	}

	if len(p.CorrelationData) > 0 {
		if count, err = p.CorrelationData.WriteToAsProperty(0x09, w); err != nil {
			return 0, err
		}
		n += count
	}

	for k, v := range p.UserProperties {
		if err = primitives.WriteByte(0x26, w); err != nil {
			return 0, err
		}
		n++

		if count, err = k.WriteTo(w); err != nil {
			return 0, err
		}
		n += count

		if count, err = v.WriteTo(w); err != nil {
			return 0, err
		}
		n += count
	}

	if p.SubscriptionIdentifier > 0 {
		if count, err = p.SubscriptionIdentifier.WriteToAsProperty(0x0B, w); err != nil {
			return 0, err
		}
		n += count
	}

	if len(p.ContentType) > 0 {
		if count, err = p.ContentType.WriteToAsProperty(0x03, w); err != nil {
			return 0, err
		}
		n += count
	}
	/* Properties end */

	//Finally, write the payload
	if len(p.Payload) > 0 {
		if count, err := primitives.Write(w, p.Payload); err != nil {
			return 0, err
		} else {
			n += int64(count)
		}
	}

	return
}

// SetAckFn sets the function to be called internally by the Ack method. This method should ONLY be called internally by
// Client when this publish is received by its Poll method.
func (p *Publish) SetAckFn(fn func(context.Context, *Publish) error) {
	if p.ackFn == nil {
		p.ackFn = fn
	}
}

// Ack will send the PUBACK control packet to the server. This method is only active when this publish has been received
// from the server and call only be called ONCE. Any subsequent calls to this method will do nothing and return no
// error.
func (p *Publish) Ack(ctx context.Context) (err error) {
	if p.ackFn != nil {
		err = p.ackFn(ctx, p)
		p.ackFn = nil
	}

	return
}
