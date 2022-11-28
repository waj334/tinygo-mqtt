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
	"io"

	"github.com/waj334/tinygo-mqtt/mqtt/packets/primitives"
)

type Subscribe struct {
	PacketIdentifier primitives.PrimitiveUint16

	/* Properties */
	SubscriptionIdentifier primitives.VariableByteInt
	UserProperties         primitives.PrimitiveStringMap

	/* Payload */
	Topics []Topic
}

func (s *Subscribe) WriteTo(w io.Writer) (n int64, err error) {
	variableHeaderLen := primitives.VariableByteInt(0)
	propertiesLen := primitives.VariableByteInt(0)
	payloadLen := primitives.VariableByteInt(0)

	// Fail early if no topics were specified
	// SPEC: The Payload MUST contain at least one Topic filter and Subscription options pair [MQTT-3.8.3-2]
	if len(s.Topics) == 0 {
		return 0, ErrControlPacketIsMalformed
	}

	// Calculate length of properties
	if s.SubscriptionIdentifier > 0 {
		propertiesLen += s.SubscriptionIdentifier.Length(true)
	}

	for k, v := range s.UserProperties {
		//[IDENTIFIER = 1] + [STRING LENGTHS = 2+2] + [LEN(KEY STRING) = N] + [LEN(VALUE STRING) = N]
		// NOTE: User properties does not implement Primitive yet
		propertiesLen += 1 + k.Length(false) + v.Length(false)
	}

	// Calculate length of payload
	for _, topic := range s.Topics {
		payloadLen += topic.filter.Length(false) + topic.options.Length(false)
	}

	//[Packet Identifier = 2] + [PROPERTIES LENGTH = N] + [PROPERTIES = N]
	variableHeaderLen += 2 + propertiesLen.Length(false) + propertiesLen

	// Write fixed header
	fh := FixedHeader{
		Remaining: variableHeaderLen + payloadLen,
	}
	fh.SetType(SUBSCRIBE)

	// SPEC: Bits 3,2,1 and 0 of the Fixed Header of the SUBSCRIBE packet are reserved and MUST be set to 0,0,1 and 0
	//       respectively. The Server MUST treat any other value as malformed and close the Network Connection
	//       [MQTT-3.8.1-1].
	fh.SetFlags(0x02)

	var count int64
	if n, err = fh.WriteTo(w); err != nil {
		return 0, err
	}

	// Write packet identifier
	if count, err = s.PacketIdentifier.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	/* Properties begin */
	if count, err = propertiesLen.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	// Write subscription identifier
	if s.SubscriptionIdentifier > 0 {
		if count, err = s.SubscriptionIdentifier.WriteToAsProperty(0x0B, w); err != nil {
			return 0, err
		}
		n += count
	}

	// Write user properties
	for k, v := range s.UserProperties {
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
	/* Properties end */
	/* Payload begin */
	for _, topic := range s.Topics {
		if count, err = topic.filter.WriteTo(w); err != nil {
			return 0, err
		}
		n += count

		if count, err = topic.options.WriteTo(w); err != nil {
			return 0, err
		}
		n += count
	}
	/* Payload end */

	return
}
