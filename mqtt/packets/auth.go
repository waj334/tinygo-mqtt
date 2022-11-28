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

type Auth struct {
	Header                 FixedHeader
	AuthenticateReasonCode primitives.PrimitiveByte

	/* Properties */
	AuthenticationMethod primitives.PrimitiveString
	AuthenticationData   primitives.PrimitiveString
	ReasonString         primitives.PrimitiveString
	UserProperties       primitives.PrimitiveStringMap
}

func (a *Auth) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	// Read the reason code
	if _, err = a.AuthenticateReasonCode.ReadFrom(r); err != nil {
		return
	}
	n++

	if n >= int64(a.Header.Remaining) {
		return
	}

	// Read the properties
	var propertiesLen primitives.VariableByteInt
	if count, err = propertiesLen.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(a.Header.Remaining) {
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
		case 0x15: // Authentication Method
			if count, err = a.AuthenticationMethod.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x16: // Authentication Data
			if count, err = a.AuthenticationData.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x1F: // Reason String
			if count, err = a.ReasonString.ReadFrom(r); err != nil {
				return 0, err
			}
		case 0x26: // User Property
			if a.UserProperties == nil {
				a.UserProperties = make(primitives.PrimitiveStringMap)
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
			a.UserProperties[k] = v
		}
		n += count
		remaining -= count
	}

	return
}

func (a *Auth) WriteTo(w io.Writer) (n int64, err error) {
	variableHeaderLen := primitives.VariableByteInt(0)
	propertiesLen := primitives.VariableByteInt(0)

	// Calculate remaining length
	// SPEC: The Reason Code and Property Length can be omitted if the Reason Code is 0x00 (Success) and there are no
	//       Properties. In this case the AUTH has a Remaining Length of 0.
	if a.AuthenticateReasonCode != 0 {
		variableHeaderLen++
	}

	if len(a.AuthenticationMethod) > 0 {
		l := primitives.VariableByteInt(len(a.AuthenticationMethod))
		propertiesLen += 3 + l
	}

	if len(a.AuthenticationData) > 0 {
		l := primitives.VariableByteInt(len(a.AuthenticationData))
		propertiesLen += 3 + l
	}

	if len(a.ReasonString) > 0 {
		l := primitives.VariableByteInt(len(a.ReasonString))
		propertiesLen += 3 + l
	}

	for k, v := range a.UserProperties {
		propertiesLen += 1 + k.Length(false) + v.Length(false)
	}

	variableHeaderLen += propertiesLen

	a.Header.SetType(AUTH)
	a.Header.Remaining = variableHeaderLen

	var count int64
	if n, err = a.Header.WriteTo(w); err != nil {
		return 0, err
	}

	/* Properties begin */
	if len(a.AuthenticationMethod) > 0 {
		if count, err = a.AuthenticationMethod.WriteToAsProperty(0x15, w); err != nil {
			return 0, err
		}
		n += count

		if count, err = a.AuthenticationData.WriteToAsProperty(0x16, w); err != nil {
			return 0, err
		}
		n += count
	}

	if len(a.ReasonString) > 0 {
		if count, err = a.ReasonString.WriteToAsProperty(0x1F, w); err != nil {
			return 0, err
		}
		n += count
	}

	// Write user properties
	for k, v := range a.UserProperties {
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
