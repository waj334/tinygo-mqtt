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

package mqtt

import "errors"

var (
	ErrUnexpectedPacketTypeReceived = errors.New("unexpected packet type received")
	ErrClientNotConnected           = errors.New("the client is not connected")
	ErrInvalidArgument              = errors.New("invalid argument")
)

type ReasonCode byte

func (r ReasonCode) Error() string {
	switch r {
	case 0x00:
		return "success"
	case 0x80:
		return "unspecified error"
	case 0x81:
		return "malformed packet"
	case 0x82:
		return "protocol error"
	case 0x83:
		return "implementation specific error"
	case 0x84:
		return "unsupported protocol version"
	case 0x85:
		return "client identifier not valid"
	case 0x86:
		return "bad user name or password"
	case 0x87:
		return "not authorized"
	case 0x88:
		return "server not available"
	case 0x89:
		return "server busy"
	case 0x8A:
		return "banned"
	case 0x8C:
		return "bad authentication method"
	case 0x90:
		return "topic name invalid"
	case 0x95:
		return "packet too large"
	case 0x97:
		return "quota exceeded"
	case 0x99:
		return "retain not supported"
	case 0x9B:
		return "qos not supported"
	case 0x9C:
		return "use another server"
	case 0x9D:
		return "server moved"
	case 0x9F:
		return "connection rate exceeded"
	case 0xA0:
		return "maximum connect time"
	case 0xA1:
		return "subscription identifiers not supported"
	case 0xA2:
		return "wildcard subscriptions not supported"
	default:
		return "unknown error"
	}
}
