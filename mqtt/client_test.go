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

package mqtt

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestClient_matchTopic(t *testing.T) {
	type fields struct {
		conn                  net.Conn
		mutex                 sync.Mutex
		isConnected           bool
		keepAliveInterval     time.Duration
		pingRespDeadline      time.Time
		sessionExpiryInterval uint32
		eventChans            map[int]chan<- *Event
		topicChans            map[string]chan<- *Event
		responseChan          map[int]chan any
		evChanIdCounter       int
		eventMutex            sync.Mutex
		packetIdCounter       int
	}
	type args struct {
		topic  string
		filter string
	}
	tests := []struct {
		name string
		//fields fields
		args args
		want bool
	}{
		{
			name: "singleLevel0",
			args: args{
				topic:  "test",
				filter: "test",
			},
			want: true,
		},
		{
			name: "singleLevel1",
			args: args{
				topic:  "teststuff",
				filter: "test",
			},
			want: false,
		},
		{
			name: "singleLevel2",
			args: args{
				topic:  "test",
				filter: "testextra",
			},
			want: false,
		},
		{
			name: "singleLevel3",
			args: args{
				topic:  "test",
				filter: "#",
			},
			want: true,
		},
		{
			name: "singleLevel4",
			args: args{
				topic:  "test",
				filter: "+",
			},
			want: true,
		},
		{
			name: "multiLevel0",
			args: args{
				topic:  "test/extra",
				filter: "test/extra",
			},
			want: true,
		},
		{
			name: "multiLevel1",
			args: args{
				topic:  "test/extra",
				filter: "test/#",
			},
			want: true,
		},
		{
			name: "multiLevel2",
			args: args{
				topic:  "test/extra/stuff",
				filter: "#",
			},
			want: true,
		},
		{
			name: "multiLevel3",
			args: args{
				topic:  "test/extra",
				filter: "test#",
			},
			want: false,
		},
		{
			name: "multiLevel4",
			args: args{
				topic:  "test/extra/stuff",
				filter: "test/#/stuff",
			},
			want: false,
		},
		{
			name: "multiLevel5",
			args: args{
				topic:  "test/extra/stuff",
				filter: "+/extra/stuff",
			},
			want: true,
		},
		{
			name: "multiLevel6",
			args: args{
				topic:  "test/extra/stuff",
				filter: "test/+/stuff",
			},
			want: true,
		},
		{
			name: "multiLevel7",
			args: args{
				topic:  "test/extra/stuff",
				filter: "test/extra/+",
			},
			want: true,
		},
		{
			name: "multiLevel8",
			args: args{
				topic:  "test/extra/stuff",
				filter: "test/extra+",
			},
			want: false,
		},
		{
			name: "multiLevel9",
			args: args{
				topic:  "test/extra/stuff",
				filter: "+/#",
			},
			want: true,
		},
		{
			name: "multiLevel10",
			args: args{
				topic:  "test/extra/stuff",
				filter: "+#",
			},
			want: false,
		},
		{
			name: "multiLevel11",
			args: args{
				topic:  "test/extra/stuff/that/comes/after",
				filter: "+/extra/#",
			},
			want: true,
		},
		{
			name: "multiLevel12",
			args: args{
				topic:  "test/extra/stuff/that/comes/after",
				filter: "+/extra/stuff/+/comes/after",
			},
			want: true,
		},
		{
			name: "multiLevel13",
			args: args{
				topic:  "test/extra/stuff/that/comes/after",
				filter: "#/extra/stuff/dontmatter/+/after",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			/*c := &Client{
				conn:                  tt.fields.conn,
				mutex:                 tt.fields.mutex,
				isConnected:           tt.fields.isConnected,
				keepAliveInterval:     tt.fields.keepAliveInterval,
				pingRespDeadline:      tt.fields.pingRespDeadline,
				sessionExpiryInterval: tt.fields.sessionExpiryInterval,
				eventChans:            tt.fields.eventChans,
				topicChans:            tt.fields.topicChans,
				responseChan:          tt.fields.responseChan,
				evChanIdCounter:       tt.fields.evChanIdCounter,
				eventMutex:            tt.fields.eventMutex,
				packetIdCounter:       tt.fields.packetIdCounter,
			}*/

			c := &Client{}
			if got := c.matchTopic(tt.args.topic, tt.args.filter); got != tt.want {
				t.Errorf("matchTopic() = %v, want %v", got, tt.want)
			}
		})
	}
}
