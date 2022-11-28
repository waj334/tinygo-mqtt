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

import "io"

type (
	PacketType           byte
	QoS                  byte
	RetainHandlingOption byte
)

const (
	QoS0 QoS = 0
	QoS1 QoS = 1
	QoS2 QoS = 2

	SendAtTimeOfSubscribe       RetainHandlingOption = 0
	SendAtTimeOfUniqueSubscribe RetainHandlingOption = 1
	DoNotSendRetainedMessages   RetainHandlingOption = 2
)

const (
	// CONNECT - Connection request
	CONNECT PacketType = iota + 1

	// CONNACK - Connect acknowledgment
	CONNACK

	// PUBLISH - Publish message
	PUBLISH

	// PUBACK - Publish acknowledgment (QoS 1)
	PUBACK

	// PUBREC - Publish received (QoS 2 delivery part 1)
	PUBREC

	// PUBREL - Publish release (QoS 2 delivery part 2)
	PUBREL

	// PUBCOMP - Publish complete (QoS 2 delivery part 3)
	PUBCOMP

	// SUBSCRIBE - Subscribe request
	SUBSCRIBE

	// SUBACK - Subscribe Acknowledgement
	SUBACK

	// UNSUBSCRIBE - Unsubscribe request
	UNSUBSCRIBE

	// UNSUBACK - Unsubscribe acknowledgment
	UNSUBACK

	// PINGREQ - PING request
	PINGREQ

	// PINGRESP - PING response
	PINGRESP

	// DISCONNECT - Disconnect notification
	DISCONNECT

	// AUTH - Authentication exchange
	AUTH
)

type FixedHeader struct {
	Header    byte
	Remaining VariableByteInt
}

func (f *FixedHeader) SetType(packetType PacketType) {
	f.Header &= ^(f.Header & 0xF0)
	f.Header |= byte(packetType << 4)
}

func (f *FixedHeader) GetType() PacketType {
	return PacketType(f.Header >> 4)
}

func (f *FixedHeader) SetFlags(flags byte) {
	f.Header &= ^(f.Header & 0x0F)
	f.Header |= flags & 0x0F
}

func (f *FixedHeader) GetFlags() byte {
	return f.Header & 0x0F
}

func (f *FixedHeader) WriteTo(w io.Writer) (n int64, err error) {
	// Write byte 1
	if err = WriteByte(f.Header, w); err != nil {
		return 0, err
	}
	n++

	// Write the variable length
	var count int64
	if count, err = f.Remaining.WriteTo(w); err != nil {
		return 0, err
	}
	n += count

	return
}

func (f *FixedHeader) ReadFrom(r io.Reader) (n int64, err error) {
	// Read byte 1
	if f.Header, err = ReadByte(r); err != nil {
		return 0, err
	}
	n++

	var count int64
	if count, err = f.Remaining.ReadFrom(r); err != nil {
		return 0, err
	}
	n += count

	return
}
