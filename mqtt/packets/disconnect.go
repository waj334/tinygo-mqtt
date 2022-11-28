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

type Disconnect struct {
	Header     FixedHeader
	ReasonCode primitives.PrimitiveByte

	/* Properties */
	SessionExpiryInterval primitives.PrimitiveUint32
	ReasonString          primitives.PrimitiveString
	UserProperties        primitives.PrimitiveStringMap
	ServerReference       primitives.PrimitiveString
}

func (d *Disconnect) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	/* Variable header begin */
	if count, err = d.ReasonCode.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(d.Header.Remaining) {
		return
	}

	/* Properties begin */
	var propertiesLen primitives.VariableByteInt
	if count, err = propertiesLen.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

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
		case 0x11: // Session expiry interval
			if count, err = d.SessionExpiryInterval.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x1F: // Reason String
			if count, err = d.ReasonString.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x26: // User Property
			if d.UserProperties == nil {
				d.UserProperties = make(primitives.PrimitiveStringMap)
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
			d.UserProperties[k] = v
		case 0x1C: // Server Reference
			if count, err = d.ServerReference.ReadFrom(r); err != nil {
				return 0, err
			}
		}
		n += count
		remaining -= count
	}
	/* Properties end */
	/* Variable header end */

	return
}

func (d *Disconnect) WriteTo(w io.Writer) (n int64, err error) {
	variableHeaderLen := primitives.VariableByteInt(2) // Account for reason code and properties length var
	propertiesLen := primitives.VariableByteInt(0)

	// Calculate properties length
	if d.SessionExpiryInterval > 0 {
		propertiesLen += d.SessionExpiryInterval.Length(true)
	}

	for k, v := range d.UserProperties {
		propertiesLen += 1 + k.Length(false) + v.Length(false)
	}

	if len(d.ServerReference) > 0 {
		propertiesLen += d.ServerReference.Length(true)
	}

	variableHeaderLen += propertiesLen

	// Write fixed header
	d.Header.SetType(DISCONNECT)
	d.Header.Remaining = variableHeaderLen

	var count int64
	if n, err = d.Header.WriteTo(w); err != nil {
		return 0, err
	}

	// Write reason code
	if count, err = d.ReasonCode.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	/* Properties begin */
	if count, err = propertiesLen.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	// Session expiry interval
	if d.SessionExpiryInterval > 0 {
		if count, err = d.SessionExpiryInterval.WriteToAsProperty(0x11, w); err != nil {
			return 0, err
		}
		n += count
	}

	// Reason string
	if len(d.ReasonString) > 0 {
		if count, err = d.ReasonString.WriteToAsProperty(0x1F, w); err != nil {
			return 0, err
		}
		n += count
	}

	// User properties
	for k, v := range d.UserProperties {
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

	return
}
