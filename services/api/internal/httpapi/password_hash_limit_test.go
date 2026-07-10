package httpapi

import (
	"testing"
	"time"
)

func TestBoundedPasswordHasherSharesConcurrencyBetweenHashAndVerify(t *testing.T) {
	delegate := &blockingPasswordHasher{
		entered: make(chan string, 3),
		release: make(chan struct{}),
	}
	hasher := newBoundedPasswordHasher(delegate, 2)
	done := make(chan error, 3)

	go func() {
		_, err := hasher.Hash("first-password")
		done <- err
	}()
	go func() {
		_, err := hasher.Verify("second-password", "encoded")
		done <- err
	}()

	<-delegate.entered
	<-delegate.entered

	go func() {
		_, err := hasher.Hash("third-password")
		done <- err
	}()

	select {
	case operation := <-delegate.entered:
		t.Fatalf("operation %q entered before a hash slot was released", operation)
	case <-time.After(20 * time.Millisecond):
	}

	delegate.release <- struct{}{}
	select {
	case <-delegate.entered:
	case <-time.After(time.Second):
		t.Fatal("third operation did not enter after a hash slot was released")
	}

	delegate.release <- struct{}{}
	delegate.release <- struct{}{}
	for range 3 {
		if err := <-done; err != nil {
			t.Fatalf("hash operation error: %v", err)
		}
	}
}

type blockingPasswordHasher struct {
	entered chan string
	release chan struct{}
}

func (hasher *blockingPasswordHasher) Hash(string) (string, error) {
	hasher.entered <- "hash"
	<-hasher.release
	return "encoded", nil
}

func (hasher *blockingPasswordHasher) Verify(string, string) (bool, error) {
	hasher.entered <- "verify"
	<-hasher.release
	return true, nil
}
