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
	"github.com/waj334/tinygo-mqtt/mqtt/packets/primitives"
	"io"
)

type Unsubscribe struct {
	PacketIdentifier primitives.PrimitiveUint16

	/* Properties */
	UserProperties primitives.PrimitiveStringMap

	/* Payload */
	Topics []Topic
}

func (u *Unsubscribe) WriteTo(w io.Writer) (n int64, err error) {
	variableHeaderLen := primitives.VariableByteInt(0)
	propertiesLen := primitives.VariableByteInt(0)
	payloadLen := primitives.VariableByteInt(0)

	// Fail early if no topics were specified
	// SPEC: The Payload of an UNSUBSCRIBE packet MUST contain at least one Topic Filter [MQTT-3.10.3-2]
	if len(u.Topics) == 0 {
		return 0, ErrControlPacketIsMalformed
	}

	// Calculate length of properties
	for k, v := range u.UserProperties {
		propertiesLen += 1 + k.Length(false) + v.Length(false)
	}

	// Calculate length of payload
	for _, topic := range u.Topics {
		payloadLen += topic.filter.Length(false)
	}

	//[Packet Identifier = 2] + [PROPERTIES LENGTH = N] + [PROPERTIES = N]
	variableHeaderLen += u.PacketIdentifier.Length(false) + propertiesLen.Length(false) + propertiesLen

	// Write fixed header
	fh := FixedHeader{
		Remaining: variableHeaderLen + payloadLen,
	}
	fh.SetType(UNSUBSCRIBE)

	// SPEC: Bits 3,2,1 and 0 of the Fixed Header of the UNSUBSCRIBE packet are reserved and MUST be set to 0,0,1 and 0
	//       respectively. The Server MUST treat any other value as malformed and close the Network Connection
	//       [MQTT-3.10.1-1].
	fh.SetFlags(0x02)

	var count int64
	if n, err = fh.WriteTo(w); err != nil {
		return 0, err
	}

	// Write packet identifier
	if count, err = u.PacketIdentifier.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	/* Properties begin */
	if count, err = propertiesLen.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	// Write user properties
	for k, v := range u.UserProperties {
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
	for _, topic := range u.Topics {
		if count, err = topic.filter.WriteTo(w); err != nil {
			return 0, err
		}
		n += count
	}
	/* Payload end */

	return
}
