/*
 * MIT License
 *
 * Copyright (c) 2022-2022 waj334
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
	"unsafe"
)

// NOTE: Borrowed from strings.Builder implementation. This is meant to avoid escape analysis when using Reader/Writer
// interfaces.
//
//go:nosplit
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

func Read(r io.Reader, b []byte) (int, error) {
	return r.Read(*(*[]byte)(noescape(unsafe.Pointer(&b))))
}

func Write(w io.Writer, b []byte) (int, error) {
	return w.Write(*(*[]byte)(noescape(unsafe.Pointer(&b))))
}

////////////////////////////////////////////////////////////

type VariableByteInt uint32

func (val *VariableByteInt) Length() VariableByteInt {
	if *val < 128 {
		return 1
	} else if *val < 16_384 {
		return 2
	} else if *val < 2_097_152 {
		return 3
	} else if *val <= 268_435_455 {
		return 4
	}

	// Overflow
	return 0
}

func (val *VariableByteInt) WriteTo(w io.Writer) (int64, error) {
	result := make([]byte, 0, 4)
	var output []byte
	i := 0

	for {
		digit := byte(*val % 128)
		*val /= 128
		if *val > 0 {
			digit |= 0x80
		}

		// Re-slice result to make room for the additional byte
		output = result[:i+1]

		// Set the byte
		output[i] = digit
		i++

		if *val == 0 {
			break
		}
	}

	n, err := Write(w, output)
	if err != nil {
		return 0, err
	}

	return int64(n), nil
}

func (val *VariableByteInt) ReadFrom(r io.Reader) (n int64, err error) {
	var multiplier uint32
	for multiplier < 27 { // fix: Infinite '(digit & 128) == 1' will cause the dead loop
		var digit byte
		if digit, err = ReadByte(r); err != nil {
			return 0, err
		}
		n++

		*val |= VariableByteInt(digit&127) << multiplier
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}

	return
}

func ReadStringFrom(r io.Reader) (str string, err error) {
	lenBuf := make([]byte, 2)

	// Read byte-by-byte to avoid heap escape
	if lenBuf[0], err = ReadByte(r); err != nil {
		return
	}
	if lenBuf[1], err = ReadByte(r); err != nil {
		return
	}

	length := binary.BigEndian.Uint16(lenBuf)

	buf := make([]byte, length)
	if _, err := r.Read(buf); err != nil {
		return "", err
	}

	return string(buf), nil
}

func WriteStringTo(val string, w io.Writer) (n int, err error) {
	if err = WriteUint16(uint16(len(val)), w); err != nil {
		return 0, err
	}
	n += 2

	count, err := io.WriteString(w, val)
	if err != nil {
		return 0, err
	}
	n += count
	return
}

//go:inline
func WriteByte(b byte, w io.Writer) error {
	_, err := Write(w, unsafe.Slice(&b, 1))
	return err
}

//go:inline
func ReadByte(r io.Reader) (b byte, err error) {
	_, err = Read(r, unsafe.Slice(&b, 1))
	return
}

//go:inline
func WriteUint16(val uint16, w io.Writer) (err error) {
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&val)), 2)
	binary.BigEndian.PutUint16(buf, val)
	_, err = Write(w, buf)
	return
}

//go:inline
func WriteUint32(val uint32, w io.Writer) (err error) {
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&val)), 4)
	binary.BigEndian.PutUint32(buf, val)
	_, err = Write(w, buf)
	return
}

func WriteStringProperty(identifier byte, val string, w io.Writer) (err error) {
	if err = WriteByte(identifier, w); err != nil {
		return
	}
	if _, err = WriteStringTo(val, w); err != nil {
		return
	}
	return nil
}

func WriteBytesProperty(identifier byte, val []byte, w io.Writer) (err error) {
	if err = WriteByte(identifier, w); err != nil {
		return
	}
	if err = WriteUint16(uint16(len(val)), w); err != nil {
		return err
	}
	if _, err = Write(w, val); err != nil {
		return
	}
	return nil
}

func WriteByteProperty(identifier byte, val byte, w io.Writer) (err error) {
	if err = WriteByte(identifier, w); err != nil {
		return
	}
	if err = WriteByte(val, w); err != nil {
		return
	}
	_, err = Write(w, []byte{identifier, val})
	return
}

func WriteUint16Property(identifier byte, val uint16, w io.Writer) (err error) {
	if err = WriteByte(identifier, w); err != nil {
		return
	}
	if err = WriteUint16(val, w); err != nil {
		return
	}
	return nil
}

func WriteUint32Property(identifier byte, val uint32, w io.Writer) (err error) {
	if err = WriteByte(identifier, w); err != nil {
		return
	}
	if err = WriteUint32(val, w); err != nil {
		return
	}
	return nil
}
