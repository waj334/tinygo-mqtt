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

type Auth struct {
	Header                 FixedHeader
	AuthenticateReasonCode byte

	/* Properties */
	AuthenticationMethod string
	AuthenticationData   []byte
	ReasonString         string
	UserProperties       map[string]string
}

func (a *Auth) WriteTo(w io.Writer) (n int64, err error) {
	variableHeaderLen := VariableByteInt(0)
	propertiesLen := VariableByteInt(0)

	// Calculate remaining length
	// SPEC: The Reason Code and Property Length can be omitted if the Reason Code is 0x00 (Success) and there are no
	//       Properties. In this case the AUTH has a Remaining Length of 0.
	if a.AuthenticateReasonCode != 0 {
		variableHeaderLen++
	}

	if len(a.AuthenticationMethod) > 0 {
		l := VariableByteInt(len(a.AuthenticationMethod))
		propertiesLen += 3 + l
	}

	if len(a.AuthenticationData) > 0 {
		l := VariableByteInt(len(a.AuthenticationData))
		propertiesLen += 3 + l
	}

	if len(a.ReasonString) > 0 {
		l := VariableByteInt(len(a.ReasonString))
		propertiesLen += 3 + l
	}

	for k, v := range a.UserProperties {
		//[IDENTIFIER = 1] + [STRING LENGTHS = 2+2] + [LEN(KEY STRING) = N] + [LEN(VALUE STRING) = N]
		keyLen := VariableByteInt(len(k))
		valueLen := VariableByteInt(len(v))
		propertiesLen += 5 + keyLen + valueLen
	}

	variableHeaderLen += propertiesLen

	a.Header.SetType(AUTH)
	a.Header.Remaining = variableHeaderLen
	if n, err = a.Header.WriteTo(w); err != nil {
		return 0, err
	}

	/* Properties begin */
	if len(a.AuthenticationMethod) > 0 {
		if err = WriteStringProperty(0x15, a.AuthenticationMethod, w); err != nil {
			return 0, err
		}
	}

	if len(a.AuthenticationData) > 0 {
		if err = WriteBytesProperty(0x15, a.AuthenticationData, w); err != nil {
			return 0, err
		}
	}

	if len(a.ReasonString) > 0 {
		if err = WriteStringProperty(0x15, a.ReasonString, w); err != nil {
			return 0, err
		}
	}

	// Write user properties
	for k, v := range a.UserProperties {
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

func (a *Auth) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	// Read the reason code
	if a.AuthenticateReasonCode, err = ReadByte(r); err != nil {
		return
	}
	n++

	if n >= int64(a.Header.Remaining) {
		return
	}

	// Read the properties
	var propertiesLen VariableByteInt
	if count, err = propertiesLen.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(a.Header.Remaining) {
		return
	}

	remaining := int(propertiesLen)
	for remaining > 0 {
		// Read the identifier byte
		var identifier byte
		if identifier, err = ReadByte(r); err != nil {
			return 0, err
		}
		remaining--

		switch identifier {
		case 0x15: // Authentication Method
			if a.AuthenticationMethod, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			remaining -= 2 + len(a.AuthenticationMethod)
		case 0x16: // Authentication Data
			var bytesLen uint16
			if err = binary.Read(r, binary.BigEndian, &bytesLen); err != nil {
				return 0, err
			}

			a.AuthenticationData = make([]byte, bytesLen)
			if _, err = Read(r, a.AuthenticationData); err != nil {
				return 0, err
			}
			remaining -= 2 + len(a.AuthenticationData)
		case 0x1F: // Reason String
			if a.ReasonString, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			remaining -= 2 + len(a.ReasonString)
		case 0x26: // User Property
			if a.UserProperties == nil {
				a.UserProperties = make(map[string]string)
			}
			var k, v string
			if k, err = ReadStringFrom(r); err != nil {
				return 0, err
			}

			if v, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			a.UserProperties[k] = v
			remaining -= 4 + len(k) + len(v)
		}
	}

	n += int64(propertiesLen)
	return
}
