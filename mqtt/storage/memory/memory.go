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

package memory

import (
	"github.com/waj334/tinygo-mqtt/mqtt/storage"
	"sync"
)

// Storage keeps the pointer the existing control packets alive by storing them in an internal map. The storage
// implementation will not continue to persist any of its contents after  a restart (or power cycle).
type Storage struct {
	// Use a slice to preserve order
	store []entry

	mutex sync.Mutex
}

type entry struct {
	id     uint16
	packet any
}

func NewStorage() *Storage {
	return &Storage{}
}

// Store stores a pointer to the control packet.
func (s *Storage) Store(identifier uint16, packet any) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Ensure that this control packet is NOT already stored
	for _, e := range s.store {
		if e.id == identifier {
			return storage.ErrDuplicateEntry
		}
	}

	// Create a new entry for this control packet
	newEntry := entry{
		id:     identifier,
		packet: packet,
	}

	// Append the new entry to the end of the slice.
	s.store = append(s.store, newEntry)

	return
}

// Get returns a pointer to a control packet from the storage with the specified identifier.
func (s *Storage) Get(identifier uint16) (packet any, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, e := range s.store {
		if e.id == identifier {
			return e.packet, nil
		}
	}

	// No entry was found
	return nil, storage.ErrNoEntry
}

// Drop removes the control packet from the storage
func (s *Storage) Drop(identifier uint16) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, e := range s.store {
		if e.id == identifier {
			// Remove the entry by not including it in the re-slice domains.
			s.store = append(s.store[:i], s.store[i+1:]...)
			return
		}
	}

	// No entry was found
	return storage.ErrNoEntry
}
