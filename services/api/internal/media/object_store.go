package media

import (
	"context"
	"errors"
	"io"
)

var (
	ErrObjectNotFound    = errors.New("object not found")
	ErrObjectExists      = errors.New("object already exists")
	ErrObjectUnavailable = errors.New("object store unavailable")
	ErrObjectIntegrity   = errors.New("object integrity error")
	ErrInvalidObjectKey  = errors.New("invalid object key")
)

type ObjectKey string
type PutObject struct {
	Key         ObjectKey
	Body        io.ReadSeeker
	Size        int64
	SHA256      [32]byte
	ContentType string
}
type Object struct {
	Body   io.ReadCloser
	Size   int64
	SHA256 [32]byte
}
type ObjectStore interface {
	Put(context.Context, PutObject) error
	Open(context.Context, ObjectKey) (Object, error)
	Delete(context.Context, ObjectKey) error
}
type ReadinessChecker interface {
	VerifyReadiness(context.Context) error
}
