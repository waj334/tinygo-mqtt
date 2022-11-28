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
	"unsafe"
)

type Primitive interface {
	WriteTo(w io.Writer) (n int64, err error)
	WriteToAsProperty(identifier byte, w io.Writer) (n int64, err error)
	ReadFrom(r io.Reader) (n int64, err error)
	Length(property bool) VariableByteInt
}

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
