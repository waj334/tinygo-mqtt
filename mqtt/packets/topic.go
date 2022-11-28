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

package packets

type Topic struct {
	filter  string
	options byte
}

func (t *Topic) SetQoS(qos QoS) *Topic {
	// Set bits [1 - 0]
	t.options &= ^byte(1 << 0)
	t.options &= ^byte(1 << 1)
	t.options |= byte(qos)
	return t
}

func (t *Topic) Filter() string {
	return t.filter
}

func (t *Topic) SetFilter(filter string) *Topic {
	if len(t.filter) >= 6 && t.filter[:6] == "$share" {
		// Unset no local option
		// SPEC: It is a Protocol Error to set the No Local bit to 1 on a Shared Subscription [MQTT-3.8.3-4]
		t.SetNoLocal(false)
	}
	t.filter = filter
	return t
}

func (t *Topic) SetNoLocal(on bool) *Topic {
	// Set bit 2
	t.options &= ^byte(1 << 2)

	// Leave bit unset if this is a shared subscription
	// SPEC: It is a Protocol Error to set the No Local bit to 1 on a Shared Subscription [MQTT-3.8.3-4]
	if on && t.filter[:6] != "$share" {
		t.options |= byte(1 << 2)
	}
	return t
}

func (t *Topic) SetRetainAsPublished(on bool) *Topic {
	// Set bit 3
	t.options &= ^byte(1 << 3)
	if on {
		t.options |= byte(1 << 3)
	}
	return t
}

func (t *Topic) SetRetainHandling(handling RetainHandlingOption) *Topic {
	// Set bits [5 - 4]
	t.options &= ^byte(1 << 5)
	t.options &= ^byte(1 << 4)
	t.options |= byte(handling << 4)
	return t
}
