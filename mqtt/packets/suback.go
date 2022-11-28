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
	"encoding/binary"
	"io"
)

type Suback struct {
	Header           FixedHeader
	PacketIdentifier uint16

	/* Properties */
	ReasonString   string
	UserProperties map[string]string

	/* Payload */
	ReasonCodes []byte
}

func (s *Suback) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	// Fail early if no memory was allocated for reason code storage
	if len(s.ReasonCodes) == 0 {
		return 0, ErrControlPacketIsMalformed
	}

	/* Variable header begin */
	if err = binary.Read(r, binary.BigEndian, &s.PacketIdentifier); err != nil {
		return 0, err
	}
	n += 2

	if n >= int64(s.Header.Remaining) {
		return
	}
	/* Variable header end */

	/* Properties header start */
	var propertiesLen VariableByteInt
	if count, err = propertiesLen.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(s.Header.Remaining) {
		return
	}

	n += int64(propertiesLen)
	remaining := int(propertiesLen)

	for remaining > 0 {
		// Read the identifier byte
		var identifier byte
		if identifier, err = ReadByte(r); err != nil {
			return 0, err
		}
		remaining--

		switch identifier {
		case 0x1F: // Reason String
			if s.ReasonString, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			remaining -= 2 + len(s.ReasonString)
		case 0x26: // User Property
			if s.UserProperties == nil {
				s.UserProperties = make(map[string]string)
			}
			var k, v string
			if k, err = ReadStringFrom(r); err != nil {
				return 0, err
			}

			if v, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			s.UserProperties[k] = v
			remaining -= 4 + len(k) + len(v)
		}
	}

	// Fail if this is the end of the control packet
	if n >= int64(s.Header.Remaining) {
		return 0, ErrControlPacketIsMalformed
	}
	/* Properties header end */

	/* Payload begin */
	// Read the reason codes
	{
		var count int
		if count, err = io.ReadFull(r, s.ReasonCodes); err != nil {
			return 0, err
		}
		n += int64(count)
	}
	/* Payload end */

	return
}
