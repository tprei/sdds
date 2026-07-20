package media

import (
	"context"
	"errors"
	"io"
)

var ErrImageNotFound = errors.New("image not found")

type ImageMetadata struct {
	ID          string
	StorageKey  ObjectKey
	ContentType string
	ByteSize    int64
	SHA256      [32]byte
}

type ImageMetadataStore interface {
	FindAttachedImage(context.Context, string) (ImageMetadata, error)
}

type AttachedImage struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
	SHA256      [32]byte
}

type AttachedImageReader interface {
	OpenAttached(context.Context, string) (AttachedImage, error)
}

type ImageReader struct {
	metadata ImageMetadataStore
	objects  ObjectStore
}

var _ AttachedImageReader = (*ImageReader)(nil)

func NewImageReader(metadata ImageMetadataStore, objects ObjectStore) *ImageReader {
	return &ImageReader{metadata: metadata, objects: objects}
}

func (reader *ImageReader) OpenAttached(ctx context.Context, imageID string) (AttachedImage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if reader == nil || reader.metadata == nil || reader.objects == nil {
		return AttachedImage{}, ErrMediaStorageUnavailable
	}

	metadata, err := reader.metadata.FindAttachedImage(ctx, imageID)
	if err != nil {
		if errors.Is(err, ErrImageNotFound) {
			return AttachedImage{}, ErrImageNotFound
		}
		return AttachedImage{}, err
	}
	if metadata.ID == "" || metadata.StorageKey == "" || metadata.ContentType == "" || metadata.ByteSize <= 0 {
		return AttachedImage{}, ErrMediaIntegrity
	}

	object, err := reader.objects.Open(ctx, metadata.StorageKey)
	if err != nil {
		closeImageBody(object.Body)
		return AttachedImage{}, mapAttachedObjectError(err)
	}
	if object.Body == nil {
		return AttachedImage{}, ErrMediaIntegrity
	}
	if object.Size != metadata.ByteSize || object.SHA256 != metadata.SHA256 {
		closeImageBody(object.Body)
		return AttachedImage{}, ErrMediaIntegrity
	}

	return AttachedImage{
		Body:        object.Body,
		ContentType: metadata.ContentType,
		Size:        metadata.ByteSize,
		SHA256:      metadata.SHA256,
	}, nil
}

func closeImageBody(body io.ReadCloser) {
	if body != nil {
		_ = body.Close()
	}
}

func mapAttachedObjectError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, ErrObjectNotFound), errors.Is(err, ErrObjectIntegrity), errors.Is(err, ErrInvalidObjectKey):
		return ErrMediaIntegrity
	default:
		return ErrMediaStorageUnavailable
	}
}
