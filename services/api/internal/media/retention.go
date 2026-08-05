package media

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// DefaultRetentionSweepInterval is how often a running API reclaims object
// bytes for uploads that expired or were detached from a deleted note. Without
// it the only sweep is the one at startup, so a deleted note's bytes would sit
// in the object store until the next deploy.
const DefaultRetentionSweepInterval = 5 * time.Minute

// expiredCleaner is the slice of UploadService the sweeper drives.
type expiredCleaner interface {
	CleanupExpired(ctx context.Context, now time.Time) error
}

// RetentionSweeper runs a cleaner on a fixed interval until Run's context is
// cancelled. It owns no goroutine of its own: the caller decides where Run
// blocks and when it stops.
type RetentionSweeper struct {
	cleaner  expiredCleaner
	interval time.Duration
	now      func() time.Time
}

// NewRetentionSweeper builds a sweeper for cleaner. A non-positive interval
// falls back to DefaultRetentionSweepInterval; a nil clock uses time.Now.
func NewRetentionSweeper(cleaner expiredCleaner, interval time.Duration, now func() time.Time) *RetentionSweeper {
	if interval <= 0 {
		interval = DefaultRetentionSweepInterval
	}
	if now == nil {
		now = time.Now
	}
	return &RetentionSweeper{cleaner: cleaner, interval: interval, now: now}
}

// Run blocks until ctx is cancelled, sweeping on each tick. It does not sweep
// on entry: startup already ran CleanupExpired before serving began. A failed
// sweep is logged and retried on the next tick; retention is best-effort and
// must never stop the process.
func (sweeper *RetentionSweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(sweeper.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := sweeper.cleaner.CleanupExpired(ctx, sweeper.now()); err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("upload retention sweep failed", "error", err)
			}
		}
	}
}
