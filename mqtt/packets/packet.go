package packets

import "io"

type (
	PacketType   byte
	PacketHeader [2]byte
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

	//DISCONNECT - Disconnect notification
	DISCONNECT

	// AUTH - Authentication exchange
	AUTH
)

type FixedHeader struct {
	Header    byte
	Remaining int
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

	// Write the variable length
	var count int
	if count, err = WriteVariableByteInt(f.Remaining, w); err != nil {
		return 0, err
	}

	n += 1 + int64(count) // Account for byte 1
	return
}

func (f *FixedHeader) ReadFrom(r io.Reader) (n int64, err error) {
	// Read byte 1
	if f.Header, err = ReadByte(r); err != nil {
		return 0, err
	}

	if f.Remaining, n, err = ReadVariableByteInt(r); err != nil {
		return 0, err
	}

	n++ // Account for byte 1
	return
}
