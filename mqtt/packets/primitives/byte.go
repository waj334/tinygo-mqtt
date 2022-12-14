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
	"io"
)

type PrimitiveByte byte

func (p *PrimitiveByte) WriteTo(w io.Writer) (n int64, err error) {
	// Write the length of the string
	if err = WriteByte(byte(*p), w); err != nil {
		return 0, err
	}
	n = 1

	return
}

func (p *PrimitiveByte) WriteToAsProperty(identifier byte, w io.Writer) (n int64, err error) {
	if err = WriteByte(identifier, w); err != nil {
		return 0, err
	}

	if _, err = p.WriteTo(w); err != nil {
		return 0, err
	}
	n = 2

	return
}

func (p *PrimitiveByte) ReadFrom(r io.Reader) (n int64, err error) {
	var b byte
	if b, err = ReadByte(r); err != nil {
		return 0, err
	}
	n = 1

	*p = PrimitiveByte(b)

	return
}

func (p *PrimitiveByte) Length(property bool) (result VariableByteInt) {
	result = 1
	if property {
		result++
	}
	return
}

func (p *PrimitiveByte) Bool() bool {
	return *p != 0
}

func (p *PrimitiveByte) Value() byte {
	return byte(*p)
}
