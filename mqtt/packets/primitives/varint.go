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

import "io"

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
	var output []byte
	i := 0

	for {
		digit := byte(*v % 128)
		*v /= 128
		if *v > 0 {
			digit |= 0x80
		}

		// Re-slice result to make room for the additional byte
		output = result[:i+1]

		// Set the byte
		output[i] = digit
		i++

		if *v == 0 {
			break
		}
	}

	n, err := Write(w, output)
	if err != nil {
		return 0, err
	}

	return int64(n), nil
}

func (v *VariableByteInt) WriteToAsProperty(identifier byte, w io.Writer) (n int64, err error) {
	if err = WriteByte(identifier, w); err != nil {
		return 0, err
	}

	if _, err = v.WriteTo(w); err != nil {
		return 0, err
	}
	n = 2

	return
}

func (v *VariableByteInt) ReadFrom(r io.Reader) (n int64, err error) {
	var multiplier uint32
	for multiplier < 27 { // fix: Infinite '(digit & 128) == 1' will cause the dead loop
		var digit byte
		if digit, err = ReadByte(r); err != nil {
			return 0, err
		}
		n++

		*v |= VariableByteInt(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}

	return
}
