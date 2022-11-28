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
	KeepAlive    uint16
	ClientId     string
	Username     string
	Password     string

	WillRetain bool
	WillQos    byte
	Will       []byte
	WillTopic  string

	/* Will properties */
	WillDelayInterval         uint32
	WillMessageExpiryInterval uint32
	WillContentType           string
	WillResponseTopic         string
	WillCorrelationData       []byte
	WillUserProperties        map[string]string

	/* Variable header properties */
	RequestResponseInformation byte
	RequestProblemInformation  byte
	ReceiveMaximum             uint16
	TopicAliasMaximum          uint16
	SessionExpiryInterval      uint32
	MaximumPacketSize          uint32
	UserProperties             map[string]string
	AuthenticationMethod       string
	AuthenticationData         []byte
}

func (c *Connect) WriteTo(w io.Writer) (n int64, err error) {
	var flags byte
	variableHeaderLen := 11
	propertiesLen := 0
	willPropertiesLen := 0
	payloadLen := len(c.ClientId) + 2

	// Calculate length of properties and payload
	if c.SessionExpiryInterval > 0 {
		variableHeaderLen += 5
	}

	if c.ReceiveMaximum > 0 {
		variableHeaderLen += 3
	}

	if c.MaximumPacketSize > 0 {
		variableHeaderLen += 5
	}

	if c.TopicAliasMaximum > 0 {
		variableHeaderLen += 3
	}

	if c.RequestResponseInformation > 0 {
		variableHeaderLen += 2
	}

	if c.RequestProblemInformation > 0 {
		variableHeaderLen += 2
	}

	for k, v := range c.UserProperties {
		variableHeaderLen += 5 + len(k) + len(v)
	}

	if len(c.AuthenticationMethod) > 0 {
		variableHeaderLen += 6 + len(c.AuthenticationMethod) + len(c.AuthenticationData)
	}

	// Set flags bits
	if c.CleanSession {
		// Set bit 1
		flags |= 1 << 1
	}

	if len(c.Username) > 0 {
		// Set bit 7
		flags |= 1 << 7
		payloadLen += 2 + len(c.Username)
	}

	if len(c.Password) > 0 {
		// Set bit 6
		flags |= 1 << 6
		payloadLen += 2 + len(c.Password)
	}

	if len(c.Will) > 0 {
		// Set bit 2
		flags |= 1 << 2

		if c.WillRetain {
			// Set bit 5
			flags |= 1 << 5
		}

		// Will QoS is bits 4 and 3
		flags |= c.WillQos << 3

		// The will and will topic are actually a part of the payload
		payloadLen += 2 + len(c.Will) + 1 // Includes byte for payload property indicator
		payloadLen += 2 + len(c.WillTopic)

		if c.WillDelayInterval > 0 {
			willPropertiesLen += 5
		}

		if c.WillMessageExpiryInterval > 0 {
			willPropertiesLen += 5
		}

		if len(c.WillContentType) > 0 {
			willPropertiesLen += 3 + len(c.WillContentType)
		}

		if len(c.WillResponseTopic) > 0 {
			willPropertiesLen += 3 + len(c.WillResponseTopic)
		}

		if len(c.WillCorrelationData) > 0 {
			willPropertiesLen += 3 + len(c.WillCorrelationData)
		}

		for k, v := range c.WillUserProperties {
			willPropertiesLen += 5 + len(k) + len(v)
		}
	}

	payloadLen += willPropertiesLen

	/* Fixed header begin */
	fh := FixedHeader{
		Remaining: variableHeaderLen + payloadLen,
	}
	fh.SetType(CONNECT)

	if n, err = fh.WriteTo(w); err != nil {
		return 0, err
	}

	/* Fixed header end */

	/* Variable header begin */
	if _, err = WriteStringTo("MQTT", w); err != nil {
		return 0, err
	}

	if err = WriteByte(byte(c.Version), w); err != nil {
		return 0, err
	}

	if err = WriteByte(flags, w); err != nil {
		return 0, err
	}

	if err = WriteUint16(c.KeepAlive, w); err != nil {
		return 0, err
	}

	if _, err = WriteVariableByteInt(propertiesLen, w); err != nil {
		return 0, err
	}

	if c.SessionExpiryInterval > 0 {
		if err = WriteUint32Property(0x11, c.SessionExpiryInterval, w); err != nil {
			return 0, err
		}
	}

	if c.ReceiveMaximum > 0 {
		if err = WriteUint16Property(0x21, c.ReceiveMaximum, w); err != nil {
			return 0, err
		}
	}

	if c.MaximumPacketSize > 0 {
		if err = WriteUint32Property(0x27, c.MaximumPacketSize, w); err != nil {
			return 0, err
		}
	}

	if c.TopicAliasMaximum > 0 {
		if err = WriteUint16Property(0x12, c.TopicAliasMaximum, w); err != nil {
			return 0, err
		}
	}

	if c.RequestResponseInformation > 0 {
		if err = WriteByteProperty(0x19, c.RequestResponseInformation, w); err != nil {
			return 0, err
		}
	}

	if c.RequestProblemInformation > 0 {
		if err = WriteByteProperty(0x19, c.RequestProblemInformation, w); err != nil {
			return 0, err
		}
	}

	for k, v := range c.UserProperties {
		if err = WriteByte(0x26, w); err != nil {
			return 0, err
		}

		if _, err = WriteStringTo(k, w); err != nil {
			return 0, err
		}

		if _, err = WriteStringTo(v, w); err != nil {
			return 0, err
		}
	}

	if len(c.AuthenticationMethod) > 0 {
		if err = WriteStringProperty(0x15, c.AuthenticationMethod, w); err != nil {
			return 0, err
		}

		if err = WriteBytesProperty(0x16, c.AuthenticationData, w); err != nil {
			return 0, err
		}
	}
	/* Variable header end*/

	/* Payload start */
	if _, err = WriteStringTo(c.ClientId, w); err != nil {
		return 0, err
	}

	if len(c.Will) > 0 {
		/* Will properties begin */
		if _, err = WriteVariableByteInt(willPropertiesLen, w); err != nil {
			return 0, err
		}

		// Will delay interval
		if c.WillDelayInterval > 0 {
			if err = WriteUint32Property(0x18, c.WillDelayInterval, w); err != nil {
				return 0, err
			}
		}

		// Will payload format indicator - 0x00 = Bytes
		if err = WriteByteProperty(0x01, 0x00, w); err != nil {
			return 0, err
		}

		// Will message expiry interval
		if c.WillMessageExpiryInterval > 0 {
			if err = WriteUint32Property(0x12, c.WillMessageExpiryInterval, w); err != nil {
				return 0, err
			}
		}

		// Will content type
		if len(c.WillContentType) > 0 {
			if err = WriteStringProperty(0x03, c.WillContentType, w); err != nil {
				return 0, err
			}
		}

		// Will response topic
		if len(c.WillResponseTopic) > 0 {
			if err = WriteStringProperty(0x8, c.WillResponseTopic, w); err != nil {
				return 0, err
			}
		}

		// Will correlation data
		if len(c.WillCorrelationData) > 0 {
			if err = WriteBytesProperty(0x9, c.WillCorrelationData, w); err != nil {
				return 0, err
			}
		}

		// Will user properties
		for k, v := range c.WillUserProperties {
			if err = WriteByte(0x26, w); err != nil {
				return 0, err
			}

			if _, err = WriteStringTo(k, w); err != nil {
				return 0, err
			}

			if _, err = WriteStringTo(v, w); err != nil {
				return 0, err
			}
		}

		/* Will properties end */

		// Will topic
		if _, err = WriteStringTo(c.WillTopic, w); err != nil {
			return 0, err
		}

		// Will payload
		if _, err = w.Write(c.Will); err != nil {
			return 0, err
		}
	}

	// User name
	if len(c.Username) > 0 {
		if _, err = WriteStringTo(c.Username, w); err != nil {
			return 0, err
		}
	}

	// Password
	if len(c.Password) > 0 {
		if _, err = WriteStringTo(c.Password, w); err != nil {
			return 0, err
		}
	}
	/* Payload end */

	n += int64(variableHeaderLen + propertiesLen + payloadLen)
	return
}
