package packets

import (
	"encoding/binary"
	"io"
)

type Connack struct {
	Header     FixedHeader
	Flags      byte
	ReasonCode byte

	/* Properties */
	SessionExpiryInterval   uint32
	ReceiveMaximum          uint16
	MaximumQoS              byte
	RetainAvailable         bool
	MaximumPacketSize       uint32
	ClientId                string
	TopicAliasMaximum       uint16
	ReasonString            string
	UserProperties          map[string]string
	WildcardSubscriptions   bool
	SubscriptionIdentifiers bool
	SharedSubscriptions     bool
	ServerKeepAlive         uint16
	ResponseInformation     string
	ServerReference         string
	AuthenticationMethod    string
	AuthenticationData      string
}

func (c *Connack) ReadFrom(r io.Reader) (n int64, err error) {
	var count int64

	/* Variable header begin */
	// Connect acknowledgement flags
	if c.Flags, err = ReadByte(r); err != nil {
		return 0, err
	}
	n++

	if n >= int64(c.Header.Remaining) {
		return
	}

	if c.ReasonCode, err = ReadByte(r); err != nil {
		return 0, err
	}
	n++

	if n >= int64(c.Header.Remaining) {
		return
	}

	/* Variable header end */

	/* Properties begin */
	var propertiesLen int
	if propertiesLen, count, err = ReadVariableByteInt(r); err != nil {
		return 0, err
	}
	n += count

	if n >= int64(c.Header.Remaining) {
		return
	}

	for propertiesLen > 0 {
		// Read the identifier byte
		var identifier byte
		if identifier, err = ReadByte(r); err != nil {
			return 0, err
		}
		propertiesLen--

		switch identifier {
		case 0x11: // Session Expiry Interval
			if err = binary.Read(r, binary.BigEndian, &c.SessionExpiryInterval); err != nil {
				return 0, err
			}
			propertiesLen -= 4
		case 0x21: // Receive Maximum
			if err = binary.Read(r, binary.BigEndian, &c.ReceiveMaximum); err != nil {
				return 0, err
			}
			propertiesLen -= 2
		case 0x24: // Maximum QoS
			if err = binary.Read(r, binary.BigEndian, &c.MaximumQoS); err != nil {
				return 0, err
			}
			propertiesLen -= 1
		case 0x25: // Retain Available
			var on byte
			if err = binary.Read(r, binary.BigEndian, &on); err != nil {
				return 0, err
			}
			propertiesLen -= 1
			c.RetainAvailable = on != 0
		case 0x27: // Maximum Packet Size
			if err = binary.Read(r, binary.BigEndian, &c.MaximumPacketSize); err != nil {
				return 0, err
			}
			propertiesLen -= 4
		case 0x12: // Assigned Client Identifier
			if c.ClientId, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			propertiesLen -= 2 + len(c.ClientId)
		case 0x22: // Topic Alias Maximum
			if err = binary.Read(r, binary.BigEndian, &c.TopicAliasMaximum); err != nil {
				return 0, err
			}
			propertiesLen -= 2
		case 0x1F: // Reason String
			if c.ReasonString, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			propertiesLen -= 2 + len(c.ReasonString)
		case 0x26: // User Property
			if c.UserProperties == nil {
				c.UserProperties = make(map[string]string)
			}
			var k, v string
			if k, err = ReadStringFrom(r); err != nil {
				return 0, err
			}

			if v, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			c.UserProperties[k] = v
			propertiesLen -= 4 + len(k) + len(v)
		case 0x28: // Wildcard Subscription Available
			var on byte
			if err = binary.Read(r, binary.BigEndian, &on); err != nil {
				return 0, err
			}
			propertiesLen -= 1
			c.WildcardSubscriptions = on != 0
		case 0x29: // Subscription Identifiers Available
			var on byte
			if err = binary.Read(r, binary.BigEndian, &on); err != nil {
				return 0, err
			}
			propertiesLen -= 1
			c.SubscriptionIdentifiers = on != 0
		case 0x2A: // Shared Subscription Available
			var on byte
			if err = binary.Read(r, binary.BigEndian, &on); err != nil {
				return 0, err
			}
			propertiesLen -= 1
			c.SharedSubscriptions = on != 0
		case 0x13: // Server Keep Alive
			if err = binary.Read(r, binary.BigEndian, &c.ServerKeepAlive); err != nil {
				return 0, err
			}
			propertiesLen -= 2
		case 0x1A: // Response Information
			if c.ResponseInformation, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			propertiesLen -= 2 + len(c.ResponseInformation)
		case 0x1C: // Server Reference
			if c.ServerReference, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			propertiesLen -= 2 + len(c.ServerReference)
		case 0x15: // Authentication Method
			if c.AuthenticationMethod, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			propertiesLen -= 2 + len(c.AuthenticationMethod)
		case 0x16: // Authentication Data
			if c.AuthenticationData, err = ReadStringFrom(r); err != nil {
				return 0, err
			}
			propertiesLen -= 2 + len(c.AuthenticationData)
		}
	}

	n += int64(propertiesLen)
	/* Properties end */
	return
}
