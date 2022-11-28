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

package packets

import (
	"io"

	"github.com/waj334/tinygo-mqtt/mqtt/packets/primitives"
)

type ProtocolVersion byte

const (
	MQTT5   ProtocolVersion = 5
	MQTT311 ProtocolVersion = 4
	MQTT31  ProtocolVersion = 3
)

type Connect struct {
	Version      ProtocolVersion
	CleanSession bool
	KeepAlive    primitives.PrimitiveUint16
	ClientId     primitives.PrimitiveString
	Username     primitives.PrimitiveString
	Password     primitives.PrimitiveString

	WillRetain bool
	WillQos    QoS
	Will       primitives.PrimitiveString
	WillTopic  primitives.PrimitiveString

	/* Will properties */
	WillDelayInterval         primitives.PrimitiveUint32
	WillMessageExpiryInterval primitives.PrimitiveUint32
	WillContentType           primitives.PrimitiveString
	WillResponseTopic         primitives.PrimitiveString
	WillCorrelationData       primitives.PrimitiveString
	WillUserProperties        primitives.PrimitiveStringMap

	/* Variable header properties */
	RequestResponseInformation primitives.PrimitiveByte
	RequestProblemInformation  primitives.PrimitiveByte
	ReceiveMaximum             primitives.PrimitiveUint16
	TopicAliasMaximum          primitives.PrimitiveUint16
	SessionExpiryInterval      primitives.PrimitiveUint32
	MaximumPacketSize          primitives.PrimitiveUint32
	UserProperties             primitives.PrimitiveStringMap
	AuthenticationMethod       primitives.PrimitiveString
	AuthenticationData         primitives.PrimitiveString
}

func (c *Connect) WriteTo(w io.Writer) (n int64, err error) {
	var flags primitives.PrimitiveByte
	variableHeaderLen := primitives.VariableByteInt(11)
	propertiesLen := primitives.VariableByteInt(0)
	willPropertiesLen := primitives.VariableByteInt(0)
	payloadLen := c.ClientId.Length(false)

	// Calculate length of properties and payload
	if c.SessionExpiryInterval > 0 {
		propertiesLen += c.SessionExpiryInterval.Length(true)
	}

	if c.ReceiveMaximum > 0 {
		propertiesLen += c.ReceiveMaximum.Length(true)
	}

	if c.MaximumPacketSize > 0 {
		propertiesLen += c.MaximumPacketSize.Length(true)
	}

	if c.TopicAliasMaximum > 0 {
		propertiesLen += c.TopicAliasMaximum.Length(true)
	}

	if c.RequestResponseInformation > 0 {
		propertiesLen += c.RequestResponseInformation.Length(true)
	}

	if c.RequestProblemInformation > 0 {
		propertiesLen += c.RequestProblemInformation.Length(true)
	}

	for k, v := range c.UserProperties {
		propertiesLen += 1 + k.Length(false) + v.Length(false)
	}

	if len(c.AuthenticationMethod) > 0 {
		propertiesLen += c.AuthenticationMethod.Length(true)
		propertiesLen += c.AuthenticationData.Length(true)
	}

	variableHeaderLen += propertiesLen

	// Set flags bits
	if c.CleanSession {
		// Set bit 1
		flags |= 1 << 1
	}

	if len(c.Username) > 0 {
		// Set bit 7
		flags |= 1 << 7
		payloadLen += c.Username.Length(false)
	}

	if len(c.Password) > 0 {
		// Set bit 6
		flags |= 1 << 6
		payloadLen += c.Password.Length(false)
	}

	if len(c.Will) > 0 {
		// Set bit 2
		flags |= 1 << 2

		if c.WillRetain {
			// Set bit 5
			flags |= 1 << 5
		}

		// Will QoS is bits 4 and 3
		flags |= primitives.PrimitiveByte(c.WillQos) << 3

		// The will and will topic are actually a part of the payload
		payloadLen += 1 + c.Will.Length(false) // Includes byte for payload property indicator
		payloadLen += c.WillTopic.Length(false)

		if c.WillDelayInterval > 0 {
			willPropertiesLen += c.WillDelayInterval.Length(true)
		}

		if c.WillMessageExpiryInterval > 0 {
			willPropertiesLen += c.WillMessageExpiryInterval.Length(true)
		}

		if len(c.WillContentType) > 0 {
			willPropertiesLen += c.WillContentType.Length(true)
		}

		if len(c.WillResponseTopic) > 0 {
			willPropertiesLen += c.WillResponseTopic.Length(true)
		}

		if len(c.WillCorrelationData) > 0 {
			willPropertiesLen += c.WillCorrelationData.Length(true)
		}

		for k, v := range c.WillUserProperties {
			willPropertiesLen += 1 + k.Length(false) + v.Length(false)
		}
	}

	payloadLen += willPropertiesLen

	/* Fixed header begin */
	fh := FixedHeader{
		Remaining: variableHeaderLen + payloadLen,
	}
	fh.SetType(CONNECT)

	var count int64
	if n, err = fh.WriteTo(w); err != nil {
		return 0, err
	}

	/* Fixed header end */

	/* Variable header begin */
	protocolName := primitives.PrimitiveString("MQTT")
	if count, err = protocolName.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	version := primitives.PrimitiveByte(c.Version)
	if count, err = version.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	if count, err = flags.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	if count, err = c.KeepAlive.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	if count, err = propertiesLen.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	if c.SessionExpiryInterval > 0 {
		if count, err = c.SessionExpiryInterval.WriteToAsProperty(0x11, w); err != nil {
			return 0, err
		}
		n += count
	}

	if c.ReceiveMaximum > 0 {
		if count, err = c.ReceiveMaximum.WriteToAsProperty(0x21, w); err != nil {
			return 0, err
		}
		n += count
	}

	if c.MaximumPacketSize > 0 {
		if count, err = c.MaximumPacketSize.WriteToAsProperty(0x27, w); err != nil {
			return 0, err
		}
		n += count
	}

	if c.TopicAliasMaximum > 0 {
		if count, err = c.TopicAliasMaximum.WriteToAsProperty(0x12, w); err != nil {
			return 0, err
		}
		n += count
	}

	if c.RequestResponseInformation > 0 {
		if count, err = c.RequestResponseInformation.WriteToAsProperty(0x19, w); err != nil {
			return 0, err
		}
		n += count
	}

	if c.RequestProblemInformation > 0 {
		if count, err = c.RequestProblemInformation.WriteToAsProperty(0x19, w); err != nil {
			return 0, err
		}
		n += count
	}

	for k, v := range c.UserProperties {
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

	if len(c.AuthenticationMethod) > 0 {
		if count, err = c.AuthenticationMethod.WriteToAsProperty(0x15, w); err != nil {
			return 0, err
		}
		n += count

		if count, err = c.AuthenticationData.WriteToAsProperty(0x16, w); err != nil {
			return 0, err
		}
		n += count
	}
	/* Variable header end */

	/* Payload start */
	if count, err = c.ClientId.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	// SPEC: If the Will Flag is set to 1, the Will Properties is the next field in the Payload.
	//       [3.1.3.2 Will Properties]
	if len(c.Will) > 0 {
		/* Will properties begin */
		if count, err = willPropertiesLen.WriteTo(w); err != nil {
			return 0, err
		}
		n += count

		// Will delay interval
		if c.WillDelayInterval > 0 {
			if count, err = c.WillDelayInterval.WriteToAsProperty(0x18, w); err != nil {
				return 0, err
			}
			n += count
		}

		// Will payload format indicator - 0x00 = Bytes
		willPayloadFormat := primitives.PrimitiveByte(0)
		if count, err = willPayloadFormat.WriteToAsProperty(0x01, w); err != nil {
			return 0, err
		}
		n += count

		// Will message expiry interval
		if c.WillMessageExpiryInterval > 0 {
			if count, err = c.WillMessageExpiryInterval.WriteToAsProperty(0x12, w); err != nil {
				return 0, err
			}
			n += count
		}

		// Will content type
		if len(c.WillContentType) > 0 {
			if count, err = c.WillContentType.WriteToAsProperty(0x03, w); err != nil {
				return 0, err
			}
			n += count
		}

		// Will response topic
		if len(c.WillResponseTopic) > 0 {
			if count, err = c.WillResponseTopic.WriteToAsProperty(0x8, w); err != nil {
				return 0, err
			}
			n += count
		}

		// Will correlation data
		if len(c.WillCorrelationData) > 0 {
			if count, err = c.WillCorrelationData.WriteToAsProperty(0x9, w); err != nil {
				return 0, err
			}
			n += count
		}

		// Will user properties
		for k, v := range c.WillUserProperties {
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
		/* Will properties end */

		// Will topic
		if count, err = c.WillTopic.WriteTo(w); err != nil {
			return 0, err
		}
		n += count

		// Will payload
		if count, err = c.Will.WriteTo(w); err != nil {
			return 0, err
		}
		n += count
	}

	// User name
	if len(c.Username) > 0 {
		if count, err = c.Username.WriteTo(w); err != nil {
			return 0, err
		}
		n += count
	}

	// Password
	if len(c.Password) > 0 {
		if count, err = c.Password.WriteTo(w); err != nil {
			return 0, err
		}
		n += count
	}
	/* Payload end */

	return
}
