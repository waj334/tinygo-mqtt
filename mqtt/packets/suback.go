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

type Suback struct {
	Header           FixedHeader
	PacketIdentifier primitives.PrimitiveUint16

	/* Properties */
	ReasonString   primitives.PrimitiveString
	UserProperties primitives.PrimitiveStringMap

	/* Payload */
	ReasonCodes []byte
}

func (s *Suback) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	/* Variable header begin */
	if n, err = s.PacketIdentifier.ReadFrom(r); err != nil {
		return 0, err
	}

	if n >= int64(s.Header.Remaining) {
		return
	}
	/* Variable header end */

	/* Properties header start */
	var propertiesLen primitives.VariableByteInt
	if count, err = propertiesLen.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(s.Header.Remaining) {
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
		case 0x1F: // Reason String
			if count, err = s.ReasonString.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x26: // User Property
			if s.UserProperties == nil {
				s.UserProperties = make(primitives.PrimitiveStringMap)
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
			s.UserProperties[k] = v
		}
		n += count
		remaining -= count
	}

	// Fail if this is the end of the control packet
	if n >= int64(s.Header.Remaining) {
		return 0, ErrControlPacketIsMalformed
	}
	/* Properties header end */

	/* Payload begin */
	{
		var count int

		// Allocate memory for storing the reason codes
		s.ReasonCodes = make([]byte, int64(s.Header.Remaining)-n)

		// Read the reason codes
		if count, err = io.ReadFull(r, s.ReasonCodes); err != nil {
			return 0, err
		}
		n += int64(count)
	}
	/* Payload end */

	return
}
