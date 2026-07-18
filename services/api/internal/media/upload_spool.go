package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"sync"
)

var spoolSlots = make(chan struct{}, 2)

type boundedFileWriter struct {
	ctx    context.Context
	file   *os.File
	hasher hash.Hash
	size   int64
	err    error
}

type stagedFile struct {
	file    *os.File
	sha256  string
	release func()
	once    sync.Once
}

type contextFileReader struct {
	ctx  context.Context
	file io.Reader
}

func (service *UploadService) spool(ctx context.Context, receive UploadReceiver) (staged *stagedFile, id string, err error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	select {
	case spoolSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	acquired := true
	release := func() {
		if acquired {
			acquired = false
			<-spoolSlots
		}
	}
	handedOff := false
	defer func() {
		if !handedOff {
			release()
		}
	}()
	file, err := os.CreateTemp(service.config.ScratchDir, "sdds-image-upload-")
	if err != nil {
		return nil, "", errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("create image scratch file: %w", err))
	}
	staged = &stagedFile{file: file, release: release}
	defer func() {
		if !handedOff {
			if cleanupErr := staged.CloseRemove(); cleanupErr != nil {
				err = errors.Join(err, ErrMediaStorageUnavailable, cleanupErr)
			}
			staged = nil
		}
	}()
	writer := &boundedFileWriter{ctx: ctx, file: file, hasher: sha256.New()}
	id, err = receive(ctx, writer)
	if err != nil {
		return staged, "", err
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return staged, "", ctxErr
	}
	if writer.err != nil {
		return staged, "", writer.err
	}
	if writer.size == 0 {
		return staged, "", ErrInvalidMedia
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return staged, "", errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("rewind image scratch file: %w", err))
	}
	staged.sha256 = hex.EncodeToString(writer.hasher.Sum(nil))
	handedOff = true
	return staged, id, nil
}

func (writer *boundedFileWriter) Write(buffer []byte) (int, error) {
	if writer.err != nil {
		return 0, writer.err
	}
	if err := writer.ctx.Err(); err != nil {
		writer.err = err
		return 0, err
	}
	if len(buffer) == 0 {
		return 0, nil
	}
	remaining := MaxEncodedImageSize - writer.size
	if remaining <= 0 {
		writer.err = ErrMediaTooLarge
		return 0, writer.err
	}
	writeBuffer := buffer
	oversize := int64(len(buffer)) > remaining
	if oversize {
		writeBuffer = buffer[:remaining]
	}
	count, err := writer.file.Write(writeBuffer)
	if count > 0 {
		_, _ = writer.hasher.Write(writeBuffer[:count])
		writer.size += int64(count)
	}
	if err == nil && count == len(writeBuffer) {
		if oversize {
			writer.err = ErrMediaTooLarge
			return count, writer.err
		}
		if err := writer.ctx.Err(); err != nil {
			writer.err = err
			return count, err
		}
		return count, nil
	}
	if err == nil {
		err = io.ErrShortWrite
	}
	writer.err = errors.Join(ErrMediaStorageUnavailable, err)
	return count, writer.err
}

func (file *stagedFile) CloseRemove() error {
	err := errors.Join(file.file.Close(), os.Remove(file.file.Name()))
	file.once.Do(file.release)
	return err
}

func contextObjectReader(ctx context.Context, body io.ReadCloser, closeBody func() error) (io.Reader, func()) {
	ctx = nonNilContext(ctx)
	stopCh, doneCh := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(doneCh)
		select {
		case <-ctx.Done():
			_ = closeBody()
		case <-stopCh:
		}
	}()
	stop := func() {
		close(stopCh)
		<-doneCh
	}
	return contextFileReader{ctx: ctx, file: body}, stop
}

func (reader contextFileReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	count, err := reader.file.Read(buffer)
	if ctxErr := reader.ctx.Err(); ctxErr != nil {
		return count, ctxErr
	}
	return count, err
}
