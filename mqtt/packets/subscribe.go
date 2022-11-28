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
)

type Subscribe struct {
	PacketIdentifier uint16

	/* Properties */
	SubscriptionIdentifier VariableByteInt
	UserProperties         map[string]string

	/* Payload */
	Topics []Topic
}

func (s *Subscribe) WriteTo(w io.Writer) (n int64, err error) {
	variableHeaderLen := VariableByteInt(0)
	propertiesLen := VariableByteInt(0)
	payloadLen := VariableByteInt(0)

	// Fail early if no topics were specified
	// SPEC: The Payload MUST contain at least one Topic filter and Subscription options pair [MQTT-3.8.3-2]
	if len(s.Topics) == 0 {
		return 0, ErrControlPacketIsMalformed
	}

	// Calculate length of properties
	if s.SubscriptionIdentifier > 0 {
		//[IDENTIFIER = 1] + [LEN(SubscriptionIdentifier) = N]
		propertiesLen += 1 + s.SubscriptionIdentifier.Length()
	}

	for k, v := range s.UserProperties {
		//[IDENTIFIER = 1] + [STRING LENGTHS = 2+2] + [LEN(KEY STRING) = N] + [LEN(VALUE STRING) = N]
		keyLen := VariableByteInt(len(k))
		valueLen := VariableByteInt(len(v))
		propertiesLen += 5 + keyLen + valueLen
	}

	// Calculate length of payload
	for _, topic := range s.Topics {
		// [STRING LENGTH = 2] + [LEN(STRING) = N] + [OPTIONS = 1]
		filterStrLen := VariableByteInt(len(topic.filter))
		payloadLen += 3 + filterStrLen
	}

	//[Packet Identifier = 2] + [PROPERTIES LENGTH = N] + [PROPERTIES = N]
	variableHeaderLen += 2 + propertiesLen.Length() + propertiesLen

	// Write fixed header
	fh := FixedHeader{
		Remaining: variableHeaderLen + payloadLen,
	}
	fh.SetType(SUBSCRIBE)

	// SPEC: Bits 3,2,1 and 0 of the Fixed Header of the SUBSCRIBE packet are reserved and MUST be set to 0,0,1 and 0
	//       respectively. The Server MUST treat any other value as malformed and close the Network Connection
	//       [MQTT-3.8.1-1].
	fh.SetFlags(0x02)

	if n, err = fh.WriteTo(w); err != nil {
		return 0, err
	}

	// Write packet identifier
	if err = WriteUint16(s.PacketIdentifier, w); err != nil {
		return 0, err
	}

	/* Properties begin */
	if _, err = propertiesLen.WriteTo(w); err != nil {
		return 0, err
	}

	// Write subscription identifier
	if s.SubscriptionIdentifier > 0 {
		if err = WriteByte(0x0B, w); err != nil {
			return 0, err
		}

		if _, err = s.SubscriptionIdentifier.WriteTo(w); err != nil {
			return 0, err
		}
	}

	// Write user properties
	for k, v := range s.UserProperties {
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
	/* Properties end */
	/* Payload begin */
	for _, topic := range s.Topics {
		if _, err = WriteStringTo(topic.filter, w); err != nil {
			return 0, err
		}

		if err = WriteByte(topic.options, w); err != nil {
			return 0, err
		}
	}
	/* Payload end */

	n += int64(variableHeaderLen + payloadLen)
	return
}
