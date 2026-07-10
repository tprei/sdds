package httpapi

type boundedPasswordHasher struct {
	delegate passwordHasher
	slots    chan struct{}
}

func newBoundedPasswordHasher(delegate passwordHasher, concurrency int) passwordHasher {
	return boundedPasswordHasher{
		delegate: delegate,
		slots:    make(chan struct{}, concurrency),
	}
}

func (hasher boundedPasswordHasher) Hash(password string) (string, error) {
	hasher.acquire()
	defer hasher.release()

	return hasher.delegate.Hash(password)
}

func (hasher boundedPasswordHasher) Verify(password string, encoded string) (bool, error) {
	hasher.acquire()
	defer hasher.release()

	return hasher.delegate.Verify(password, encoded)
}

func (hasher boundedPasswordHasher) acquire() {
	hasher.slots <- struct{}{}
}

func (hasher boundedPasswordHasher) release() {
	<-hasher.slots
}
