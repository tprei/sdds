package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type fakeImageMetadataStore struct {
	metadata ImageMetadata
	err      error
	calls    int
}

func (fake *fakeImageMetadataStore) FindAttachedImage(context.Context, string) (ImageMetadata, error) {
	fake.calls++
	return fake.metadata, fake.err
}

type fakeImageObjectStore struct {
	object Object
	err    error
	key    ObjectKey
	calls  int
}

func (fake *fakeImageObjectStore) Put(context.Context, PutObject) error { return nil }
func (fake *fakeImageObjectStore) Open(_ context.Context, key ObjectKey) (Object, error) {
	fake.calls++
	fake.key = key
	return fake.object, fake.err
}
func (fake *fakeImageObjectStore) Delete(context.Context, ObjectKey) error { return nil }

type trackingImageBody struct {
	io.Reader
	closed bool
}

func (body *trackingImageBody) Close() error {
	body.closed = true
	return nil
}

func testImageMetadata() ImageMetadata {
	return ImageMetadata{
		ID:          "image-id",
		StorageKey:  ObjectKey("note-images/image-id"),
		ContentType: "image/jpeg",
		ByteSize:    5,
		SHA256:      [32]byte{1, 2, 3},
	}
}

func TestImageReaderReturnsNotFoundForUnknownOrUnattachedImage(t *testing.T) {
	metadata := &fakeImageMetadataStore{err: ErrImageNotFound}
	objects := &fakeImageObjectStore{}

	_, err := NewImageReader(metadata, objects).OpenAttached(context.Background(), "unknown")
	if !errors.Is(err, ErrImageNotFound) {
		t.Fatalf("OpenAttached error = %v, want %v", err, ErrImageNotFound)
	}
	if objects.calls != 0 {
		t.Fatalf("object calls = %d, want 0", objects.calls)
	}
}

func TestImageReaderReturnsAttachedImageAfterVerification(t *testing.T) {
	metadata := testImageMetadata()
	body := &trackingImageBody{Reader: bytes.NewReader([]byte("bytes"))}
	objects := &fakeImageObjectStore{object: Object{Body: body, Size: metadata.ByteSize, SHA256: metadata.SHA256}}

	got, err := NewImageReader(&fakeImageMetadataStore{metadata: metadata}, objects).OpenAttached(context.Background(), metadata.ID)
	if err != nil {
		t.Fatalf("OpenAttached error = %v", err)
	}
	if got.ContentType != metadata.ContentType || got.Size != metadata.ByteSize || got.SHA256 != metadata.SHA256 {
		t.Fatalf("attached image = %#v, want metadata fields", got)
	}
	if objects.key != metadata.StorageKey {
		t.Fatalf("object key = %q, want %q", objects.key, metadata.StorageKey)
	}
	if got.Body != body {
		t.Fatal("attached body does not own provider body")
	}
	if err := got.Body.Close(); err != nil {
		t.Fatalf("close attached body: %v", err)
	}
}

func TestImageReaderMapsMissingAttachedObjectToIntegrity(t *testing.T) {
	metadata := testImageMetadata()
	objects := &fakeImageObjectStore{err: ErrObjectNotFound}

	_, err := NewImageReader(&fakeImageMetadataStore{metadata: metadata}, objects).OpenAttached(context.Background(), metadata.ID)
	if !errors.Is(err, ErrMediaIntegrity) {
		t.Fatalf("OpenAttached error = %v, want %v", err, ErrMediaIntegrity)
	}
	if stringsContains(err.Error(), string(metadata.StorageKey)) {
		t.Fatalf("error %q exposes storage key", err)
	}
}

func TestImageReaderClosesBodyOnMetadataMismatch(t *testing.T) {
	metadata := testImageMetadata()
	tests := []struct {
		name   string
		object Object
	}{
		{
			name: "size",
			object: Object{
				Body:   &trackingImageBody{Reader: bytes.NewReader(nil)},
				Size:   metadata.ByteSize + 1,
				SHA256: metadata.SHA256,
			},
		},
		{
			name: "digest",
			object: Object{
				Body:   &trackingImageBody{Reader: bytes.NewReader(nil)},
				Size:   metadata.ByteSize,
				SHA256: [32]byte{9},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := test.object.Body.(*trackingImageBody)
			objects := &fakeImageObjectStore{object: test.object}
			_, err := NewImageReader(&fakeImageMetadataStore{metadata: metadata}, objects).OpenAttached(context.Background(), metadata.ID)
			if !errors.Is(err, ErrMediaIntegrity) {
				t.Fatalf("OpenAttached error = %v, want %v", err, ErrMediaIntegrity)
			}
			if !body.closed {
				t.Fatal("provider body was not closed")
			}
		})
	}
}

func TestImageReaderMapsTransientObjectFailureToUnavailable(t *testing.T) {
	metadata := testImageMetadata()
	objects := &fakeImageObjectStore{err: ErrObjectUnavailable}

	_, err := NewImageReader(&fakeImageMetadataStore{metadata: metadata}, objects).OpenAttached(context.Background(), metadata.ID)
	if !errors.Is(err, ErrMediaStorageUnavailable) {
		t.Fatalf("OpenAttached error = %v, want %v", err, ErrMediaStorageUnavailable)
	}
}

func stringsContains(value, fragment string) bool {
	return len(fragment) > 0 && bytes.Contains([]byte(value), []byte(fragment))
}
