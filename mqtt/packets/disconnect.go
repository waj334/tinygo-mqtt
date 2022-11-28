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
	"encoding/binary"
	"io"
)

type Disconnect struct {
	Header     FixedHeader
	ReasonCode byte

	/* Properties */
	SessionExpiryInterval uint32
	ReasonString          string
	UserProperties        map[string]string
	ServerReference       string
}

func (d *Disconnect) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	/* Variable header begin */
	if d.ReasonCode, err = ReadByte(r); err != nil {
		return 0, err
	}
	n++

	if n >= int64(d.Header.Remaining) {
		return
	}

	/* Variable header end */

	/* Properties begin */
	var propertiesLen VariableByteInt
	if count, err = propertiesLen.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	remaining := int(propertiesLen)
	for remaining > 0 {
		// Read the identifier byte
		var identifier byte
		if identifier, err = ReadByte(r); err != nil {
			return 0, err
		}
		remaining--

		switch identifier {
		case 0x11: // Session expiry interval
			if err = binary.Read(r, binary.BigEndian, &d.SessionExpiryInterval); err != nil {
				return 0, err
			}
			remaining -= 4
		case 0x1F: // Reason String
			if d.ReasonString, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			remaining -= 2 + len(d.ReasonString)
		case 0x26: // User Property
			if d.UserProperties == nil {
				d.UserProperties = make(map[string]string)
			}
			var k, v string
			if k, err = ReadStringFrom(r); err != nil {
				return 0, err
			}

			if v, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			d.UserProperties[k] = v
			remaining -= 4 + len(k) + len(v)
		case 0x1C: // Server Reference
			if d.ServerReference, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			remaining -= 2 + len(d.ServerReference)
		}
	}

	n += int64(propertiesLen)
	/* Properties end */
	return
}

func (d *Disconnect) WriteTo(w io.Writer) (n int64, err error) {
	variableHeaderLen := VariableByteInt(2) // Account for reason code and properties length var
	propertiesLen := VariableByteInt(0)

	// Calculate properties length
	if d.SessionExpiryInterval > 0 {
		propertiesLen += 5
	}

	if len(d.UserProperties) > 0 {
		propertiesLen += VariableByteInt(3 + len(d.UserProperties))
	}

	for k, v := range d.UserProperties {
		propertiesLen += VariableByteInt(5 + len(k) + len(v))
	}

	if len(d.ServerReference) > 0 {
		propertiesLen += VariableByteInt(3 + len(d.ServerReference))
	}

	variableHeaderLen += propertiesLen

	// Write fixed header
	d.Header.SetType(DISCONNECT)
	d.Header.Remaining = variableHeaderLen
	if n, err = d.Header.WriteTo(w); err != nil {
		return 0, err
	}

	// Write reason code
	if err = WriteByte(d.ReasonCode, w); err != nil {
		return 0, err
	}

	/* Properties begin */
	if _, err = propertiesLen.WriteTo(w); err != nil {
		return 0, err
	}

	// Session expiry interval
	if err = WriteUint32(d.SessionExpiryInterval, w); err != nil {
		return 0, err
	}

	// Reason string
	if _, err = WriteStringTo(d.ReasonString, w); err != nil {
		return 0, err
	}

	// User properties
	for k, v := range d.UserProperties {
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

	n += int64(variableHeaderLen)
	return
}
