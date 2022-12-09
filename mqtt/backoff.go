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

import (
	"context"
	"errors"
	"io"
	"math"
	"math/rand"
	"os"
	"time"
)

//go:inline
func backoff(ctx context.Context, fn func() error) error {
	var backoff int64
	var exponent float64
	for {
		select {
		case <-ctx.Done():
			return os.ErrDeadlineExceeded
		default:
			if err := fn(); errors.Is(err, io.EOF) {
				// Backoff
				backoff = rand.Int63n(1000) + int64(math.Pow(2, exponent)*10)
				exponent++
				time.Sleep(time.Duration(backoff) * time.Millisecond)
				continue
			} else if err != nil {
				return err
			}
		}
		return nil
	}
}
