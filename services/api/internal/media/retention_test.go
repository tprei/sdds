package media

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

var errSweepInjected = errors.New("transient object-store failure")

// recordingCleaner counts CleanupExpired calls and can inject an error on the
// first failFirst calls. It signals swept each time a sweep runs so a test can
// wait on it without sleeping a fixed wall-clock duration.
type recordingCleaner struct {
	calls     atomic.Int32
	failFirst int32
	swept     chan struct{}
}

func (cleaner *recordingCleaner) CleanupExpired(_ context.Context, _ time.Time) error {
	n := cleaner.calls.Add(1)
	if int32(n) <= cleaner.failFirst {
		return errSweepInjected
	}
	select {
	case cleaner.swept <- struct{}{}:
	default:
	}
	return nil
}

func newRecordingCleaner() *recordingCleaner {
	return &recordingCleaner{swept: make(chan struct{}, 8)}
}

func TestRetentionSweeperRunsOnTheInterval(t *testing.T) {
	cleaner := newRecordingCleaner()
	sweeper := NewRetentionSweeper(cleaner, time.Millisecond, time.Now)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sweeper.Run(ctx)
		close(done)
	}()

	// Two sweeps prove the sweeper fires on its interval rather than only once.
	for sweep := 1; sweep <= 2; sweep++ {
		select {
		case <-cleaner.swept:
		case <-time.After(time.Second):
			t.Fatalf("sweeper did not complete sweep %d within the interval", sweep)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sweeper did not stop after context cancellation")
	}
	if calls := cleaner.calls.Load(); calls < 2 {
		t.Fatalf("cleaner calls = %d, want >= 2 (periodic)", calls)
	}
}

func TestRetentionSweeperDoesNotSweepOnEntry(t *testing.T) {
	cleaner := newRecordingCleaner()
	sweeper := NewRetentionSweeper(cleaner, time.Hour, time.Now)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sweeper.Run(ctx)
		close(done)
	}()

	// Cancel well before the hour-long interval; the startup sweep already ran,
	// so Run must not sweep here.
	cancel()
	<-done
	if calls := cleaner.calls.Load(); calls != 0 {
		t.Fatalf("cleaner calls = %d, want 0 (no sweep on entry)", calls)
	}
}

func TestRetentionSweeperSurvivesACleanerError(t *testing.T) {
	cleaner := newRecordingCleaner()
	cleaner.failFirst = 1

	sweeper := NewRetentionSweeper(cleaner, time.Millisecond, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sweeper.Run(ctx)
		close(done)
	}()

	// The first sweep fails; a later sweep must still run, proving the loop
	// survives the error rather than stopping.
	select {
	case <-cleaner.swept:
	case <-time.After(2 * time.Second):
		t.Fatal("sweeper did not run again after a failed sweep")
	}

	cancel()
	<-done
	if calls := cleaner.calls.Load(); calls < 2 {
		t.Fatalf("cleaner calls = %d, want >= 2 (failure must not stop the loop)", calls)
	}
}
