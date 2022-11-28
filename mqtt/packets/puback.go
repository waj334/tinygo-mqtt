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

type Puback struct {
	Header           FixedHeader
	PacketIdentifier primitives.PrimitiveUint16
	ReasonCode       primitives.PrimitiveByte

	/* Properties */
	ReasonString   primitives.PrimitiveString
	UserProperties primitives.PrimitiveStringMap
}

// Note: The following control packets have the same structure as PUBACK:

type Pubrec struct {
	Puback
}

func (p *Pubrec) WriteTo(w io.Writer) (n int64, err error) {
	// Override the packet type in the fixed header
	p.Header.SetType(PUBREC)
	return p.Puback.WriteTo(w)
}

type Pubrel struct {
	Puback
}

func (p *Pubrel) WriteTo(w io.Writer) (n int64, err error) {
	// Override the packet type in the fixed header
	p.Header.SetType(PUBREL)

	// Pubrel has a reserved bit set to 1 in the fixed header
	p.Header.SetFlags(0x02)
	return p.Puback.WriteTo(w)
}

type Pubcomp struct {
	Puback
}

func (p *Pubcomp) WriteTo(w io.Writer) (n int64, err error) {
	// Override the packet type in the fixed header
	p.Header.SetType(PUBCOMP)
	return p.Puback.WriteTo(w)
}

////////////////

func (p *Puback) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	if n, err = p.PacketIdentifier.ReadFrom(r); err != nil {
		return 0, err
	}

	if n >= int64(p.Header.Remaining) {
		return
	}

	if count, err = p.ReasonCode.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(p.Header.Remaining) {
		return
	}

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
		case 0x1F: // Reason String
			if count, err = p.ReasonString.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x26: // User Property
			if p.UserProperties == nil {
				p.UserProperties = make(primitives.PrimitiveStringMap)
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
			p.UserProperties[k] = v
		}
		n += count
		remaining -= count
	}

	return
}

func (p *Puback) WriteTo(w io.Writer) (n int64, err error) {
	variableHeaderLen := primitives.VariableByteInt(0)
	propertiesLen := primitives.VariableByteInt(0)

	// Calculate length of variable header
	variableHeaderLen += p.PacketIdentifier.Length(false)

	if len(p.ReasonString) > 0 {
		propertiesLen += p.ReasonString.Length(true)
	}

	for k, v := range p.UserProperties {
		propertiesLen += 1 + k.Length(false) + v.Length(false)
	}

	// SPEC: Byte 3 in the Variable Header is the PUBACK Reason Code. If the Remaining Length is 2, then there is no
	//       Reason Code and the value of 0x00 (Success) is used.
	// SPEC: The Reason Code and Property Length can be omitted if the Reason Code is 0x00 (Success) and there are no
	//       Properties. In this case the PUBACK has a Remaining Length of 2.
	if p.ReasonCode != 0 || propertiesLen != 0 {
		variableHeaderLen += p.ReasonCode.Length(false)
	}

	variableHeaderLen += propertiesLen.Length(false) + propertiesLen

	// Default the packet type to PUBACK if one is not set. Possible prior values could only be PUBREL, PUBREC or
	// PUBCOMP.
	if p.Header.GetType() == 0 {
		p.Header.SetType(PUBACK)
	}
	p.Header.Remaining = variableHeaderLen

	// Write the control packet
	var count int64
	if n, err = p.Header.WriteTo(w); err != nil {
		return 0, err
	}

	if count, err = p.PacketIdentifier.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	// SPEC: Byte 3 in the Variable Header is the PUBACK Reason Code. If the Remaining Length is 2, then there is no
	//       Reason Code and the value of 0x00 (Success) is used.
	// SPEC: The Reason Code and Property Length can be omitted if the Reason Code is 0x00 (Success) and there are no
	//       Properties. In this case the PUBACK has a Remaining Length of 2.
	if p.ReasonCode != 0 || propertiesLen != 0 {
		if count, err = p.ReasonCode.WriteTo(w); err != nil {
			return 0, err
		}
		n += count
	}

	/* Properties begin */
	if count, err = propertiesLen.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	if len(p.ReasonString) > 0 {
		if count, err = p.ReasonString.WriteToAsProperty(0x1F, w); err != nil {
			return 0, err
		}
		n += count
	}

	// Write user properties
	for k, v := range p.UserProperties {
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
