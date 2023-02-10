/*
 * MIT License
 *
 * Copyright (c) 2022-2023 waj334
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
	if err = binary.Write(w, binary.BigEndian, uint16(len(*p))); err != nil {
		return 0, err
	}
	n += 2

	// Write the string
	var count int
	if count, err = w.Write([]byte(*p)); err != nil {
		return 0, err
	}
	n += int64(count)

	return
}

func (p *PrimitiveString) WriteToAsProperty(identifier byte, w io.Writer) (n int64, err error) {
	if _, err = w.Write([]byte{identifier}); err != nil {
		return 0, err
	}
	n++

	if n, err = p.WriteTo(w); err != nil {
		return 0, err
	}

	return
}

func (p *PrimitiveString) ReadFrom(r io.Reader) (n int64, err error) {
	// Read the 16-bit length
	var length uint16
	if err = binary.Read(r, binary.BigEndian, &length); err != nil {
		return 0, err
	}
	n += 2

	// Allocate memory for the string
	buf := make([]byte, length)

	// Read the string
	var count int
	if count, err = r.Read(buf); err != nil {
		return 0, err
	} else {
		n += int64(count)
	}

	*p = PrimitiveString(buf)
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
