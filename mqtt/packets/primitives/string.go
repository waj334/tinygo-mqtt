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

package primitives

import (
	"encoding/binary"
	"io"
)

type PrimitiveString string

func (p *PrimitiveString) WriteTo(w io.Writer) (n int64, err error) {
	// Write the length of the string
	if err = WriteUint16(uint16(len(*p)), w); err != nil {
		return 0, err
	}
	n += 2

	// Write the string
	count, err := Write(w, []byte(*p))
	if err != nil {
		return 0, err
	}
	n += int64(count)

	return
}

func (p *PrimitiveString) WriteToAsProperty(identifier byte, w io.Writer) (n int64, err error) {
	if err = WriteByte(identifier, w); err != nil {
		return 0, err
	}

	if n, err = p.WriteTo(w); err != nil {
		return 0, err
	}

	// Account for writing the identifier byte. Saves stack
	n++

	return
}

func (p *PrimitiveString) ReadFrom(r io.Reader) (n int64, err error) {
	lenBuf := make([]byte, 2)

	// Read byte-by-byte to avoid heap escape
	var count int
	if count, err = Read(r, lenBuf); err != nil {
		return 0, err
	}
	n += int64(count)

	// Parse the integer
	length := binary.BigEndian.Uint16(lenBuf)

	// Allocate memory for the string
	*p = PrimitiveString(make([]byte, length))
	if _, err = Read(r, []byte(*p)); err != nil {
		return 0, err
	}

	return
}

func (p *PrimitiveString) Length(property bool) (result VariableByteInt) {
	result = 2 + VariableByteInt(len(*p))
	if property {
		result++
	}
	return
}

func (p *PrimitiveString) String() string {
	return string(*p)
}
