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
	"errors"
	"io"
)

type VariableByteInt uint32

func (v *VariableByteInt) Length(property bool) (result VariableByteInt) {
	if *v < 128 {
		result = 1
	} else if *v < 16_384 {
		result = 2
	} else if *v < 2_097_152 {
		result = 3
	} else if *v <= 268_435_455 {
		result = 4
	} else {
		// Overflow
		return 0
	}

	if property {
		result++
	}

	return result
}

func (v *VariableByteInt) WriteTo(w io.Writer) (int64, error) {
	result := make([]byte, 0, 4)
	tmp := uint32(*v)

	for {
		b := byte(tmp % 128)
		tmp /= 128
		if tmp > 0 {
			b = b | 0x80
		}
		result = append(result, b)
		if tmp <= 0 {
			break
		}
	}

	count, err := w.Write(result)

	return int64(count), err
}

func (v *VariableByteInt) WriteToAsProperty(identifier byte, w io.Writer) (n int64, err error) {
	if _, err = w.Write([]byte{identifier}); err != nil {
		return 0, err
	}

	if n, err = v.WriteTo(w); err != nil {
		return 0, err
	}
	n++

	return
}

func (v *VariableByteInt) ReadFrom(r io.Reader) (n int64, err error) {
	var mul uint32
	var val uint32

	for {
		var b [1]byte
		if _, err = r.Read(b[:]); err != nil {
			return 0, err
		}
		n++

		val |= uint32(b[0]&127) << mul
		if val > 268_435_455 {
			return 0, errors.New("malformed variable byte integer")
		}

		if b[0]&128 == 0 {
			break
		}

		mul += 7
	}

	*v = VariableByteInt(val)
	return
}
