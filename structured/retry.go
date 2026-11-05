// Copyright (c) 2025 Umputun
// Derived from github.com/go-pkgz/pool (MIT License), file: middleware/middleware.go
// (Retry's exponential backoff with jitter), rewritten here as a standalone helper.
// See THIRD_PARTY_LICENSES.md for the full license text.
package structured

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"
)

// RetryConfig controls Retry.
type RetryConfig struct {
	// Attempts is the total number of calls, including the first one.
	// Values <= 0 default to 3.
	Attempts int
	// BaseDelay is the delay before the second attempt. It doubles each time.
	// Values <= 0 default to 1s.
	BaseDelay time.Duration
	// MaxDelay caps the computed delay and prevents overflow for large
	// attempt counts. Values <= 0 default to 30s.
	MaxDelay time.Duration
}

// DefaultRetryConfig returns 3 attempts, a 1s base delay and a 30s cap.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{Attempts: 3, BaseDelay: time.Second, MaxDelay: 30 * time.Second}
}

// Retry calls fn until it succeeds, the attempts are exhausted, or ctx is
// canceled. Delays grow exponentially (BaseDelay, 2x, 4x, ...) with up to 20%
// positive jitter and are capped at MaxDelay. Success returns nil; exhaustion
// returns an error wrapping the last error (inspectable with errors.Is/As);
// cancellation returns ctx.Err().
//
// fn must respect ctx, otherwise cancellation cannot interrupt a running call.
func Retry(ctx context.Context, cfg RetryConfig, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg.Attempts <= 0 {
		cfg.Attempts = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}

	var lastErr error
	for attempt := 0; attempt < cfg.Attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fn(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == cfg.Attempts-1 {
			break
		}
		if err := waitBackoff(ctx, cfg, attempt); err != nil {
			return err
		}
	}
	return fmt.Errorf("structured: retry failed after %d attempts: %w", cfg.Attempts, lastErr)
}

func waitBackoff(ctx context.Context, cfg RetryConfig, attempt int) error {
	delay := cfg.BaseDelay
	// Cap the shift before multiplying so large attempt counts cannot overflow.
	for i := 0; i < attempt && delay < cfg.MaxDelay; i++ {
		delay *= 2
	}
	if delay > cfg.MaxDelay || delay <= 0 {
		delay = cfg.MaxDelay
	}
	delay += time.Duration(float64(delay) * 0.2 * rand.Float64())

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
