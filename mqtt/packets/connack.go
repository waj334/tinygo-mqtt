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
	"github.com/waj334/tinygo-mqtt/mqtt/packets/primitives"
	"io"
)

type Connack struct {
	Header     FixedHeader
	Flags      primitives.PrimitiveByte
	ReasonCode primitives.PrimitiveByte

	/* Properties */
	SessionExpiryInterval   primitives.PrimitiveUint32
	ReceiveMaximum          primitives.PrimitiveUint16
	MaximumQoS              primitives.PrimitiveByte
	RetainAvailable         primitives.PrimitiveByte
	MaximumPacketSize       primitives.PrimitiveUint32
	ClientId                primitives.PrimitiveString
	TopicAliasMaximum       primitives.PrimitiveUint16
	ReasonString            primitives.PrimitiveString
	UserProperties          primitives.PrimitiveStringMap
	WildcardSubscriptions   primitives.PrimitiveByte
	SubscriptionIdentifiers primitives.PrimitiveByte
	SharedSubscriptions     primitives.PrimitiveByte
	ServerKeepAlive         primitives.PrimitiveUint16
	ResponseInformation     primitives.PrimitiveString
	ServerReference         primitives.PrimitiveString
	AuthenticationMethod    primitives.PrimitiveString
	AuthenticationData      primitives.PrimitiveString
}

func (c *Connack) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	/* Variable header begin */
	// Connect acknowledgement flags
	if count, err = c.Flags.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(c.Header.Remaining) {
		return
	}

	if count, err = c.ReasonCode.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(c.Header.Remaining) {
		return
	}

	/* Variable header end */

	/* Properties begin */
	var propertiesLen primitives.VariableByteInt
	if count, err = propertiesLen.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(c.Header.Remaining) {
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
		case 0x11: // Session Expiry Interval
			if count, err = c.SessionExpiryInterval.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x21: // Receive Maximum
			if count, err = c.ReceiveMaximum.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x24: // Maximum QoS
			if count, err = c.MaximumQoS.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x25: // Retain Available
			if count, err = c.RetainAvailable.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x27: // Maximum Packet Size
			if count, err = c.MaximumPacketSize.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x12: // Assigned Client Identifier
			if count, err = c.ClientId.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x22: // Topic Alias Maximum
			if count, err = c.TopicAliasMaximum.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x1F: // Reason String
			if count, err = c.ReasonString.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x26: // User Property
			if c.UserProperties == nil {
				c.UserProperties = make(primitives.PrimitiveStringMap)
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
			c.UserProperties[k] = v
		case 0x28: // Wildcard Subscription Available
			if count, err = c.WildcardSubscriptions.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x29: // Subscription Identifiers Available
			if count, err = c.SubscriptionIdentifiers.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x2A: // Shared Subscription Available
			if count, err = c.SharedSubscriptions.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x13: // Server Keep Alive
			if count, err = c.ServerKeepAlive.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x1A: // Response Information
			if count, err = c.ResponseInformation.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x1C: // Server Reference
			if count, err = c.ServerReference.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x15: // Authentication Method
			if count, err = c.AuthenticationMethod.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x16: // Authentication Data
			if count, err = c.AuthenticationData.ReadFrom(r); err != nil {
				return 0, err
			}
		}
		n += count
		remaining -= count
	}

	n += int64(c.Header.Remaining)
	/* Properties end */
	return
}
