package mqtt

import (
	"context"
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
			if err := fn(); err == io.EOF {
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
