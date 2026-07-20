package media

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"testing"
)

func TestPrepareImageUploadRejectsMediaBoundaries(t *testing.T) {
	cases := []struct {
		name string
		body func(*testing.T) []byte
		want error
	}{
		{name: "empty", body: func(_ *testing.T) []byte { return nil }, want: ErrInvalidMedia},
		{name: "truncated jpeg", body: func(t *testing.T) []byte { body := testJPEG(t, 10, 10); return body[:len(body)/2] }, want: ErrInvalidMedia},
		{name: "too wide", body: func(t *testing.T) []byte { return testPNGGray(t, MaxImageWidth+1, 1) }, want: ErrMediaDimensions},
		{name: "too tall", body: func(t *testing.T) []byte { return testPNGGray(t, 1, MaxImageHeight+1) }, want: ErrMediaDimensions},
		{name: "max area", body: func(t *testing.T) []byte { return testPNGGray(t, 4000, 4000) }},
		{name: "too much area", body: func(t *testing.T) []byte { return testPNGGray(t, 4001, 4000) }, want: ErrMediaDimensions},
	}
	for _, test := range cases {
		repo, store := newUploadRepositoryAndObjectStore()
		dir := t.TempDir()
		receipt, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: dir}).PrepareImageUpload(context.Background(), "user-1", testReceiver(test.body(t)))
		if test.want == nil && (err != nil || receipt.ImageUploadID == "") {
			t.Fatalf("%s: receipt=%+v err=%v", test.name, receipt, err)
		}
		if test.want != nil && !errors.Is(err, test.want) {
			t.Fatalf("%s: err=%v want=%v", test.name, err, test.want)
		}
		if test.want != nil {
			assertNoPublication(t, repo, store, dir)
		}
	}
}

func TestPrepareImageUploadRejectsUnsupportedMediaSignatures(t *testing.T) {
	webp := make([]byte, 20)
	copy(webp, "RIFF")
	binary.LittleEndian.PutUint32(webp[4:8], 12)
	copy(webp[8:12], "WEBP")
	for _, body := range [][]byte{[]byte("<svg></svg>"), webp} {
		repo, store := newUploadRepositoryAndObjectStore()
		dir := t.TempDir()
		_, err := newUploadService(t, repo, store, UploadConfig{ScratchDir: dir}).PrepareImageUpload(context.Background(), "user-1", testReceiver(body))
		if !errors.Is(err, ErrUnsupportedMediaType) {
			t.Fatalf("err=%v", err)
		}
		assertNoPublication(t, repo, store, dir)
	}
}

func TestInspectImageReportsSeekError(t *testing.T) {
	reader, writer, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	_ = writer.Close()
	_, err := inspectImage(context.Background(), reader)
	_ = reader.Close()
	if !errors.Is(err, ErrMediaStorageUnavailable) {
		t.Fatalf("seek error=%v", err)
	}
}
