package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"
)

var decodeSlot = make(chan struct{}, 1)

type imageMetadata struct {
	ContentType string
	ByteSize    int64
	Width       int
	Height      int
	SHA256      string
}

func inspectImage(ctx context.Context, file *os.File) (imageMetadata, error) {
	ctx = nonNilContext(ctx)
	if err := ctx.Err(); err != nil {
		return imageMetadata{}, err
	}
	info, err := file.Stat()
	if err != nil {
		return imageMetadata{}, errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("stat image scratch file: %w", err))
	}
	size := info.Size()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return imageMetadata{}, errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("rewind image for config: %w", err))
	}
	prefix := make([]byte, 512)
	reader := contextFileReader{ctx: ctx, file: file}
	prefixSize, prefixErr := reader.Read(prefix)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return imageMetadata{}, ctxErr
	}
	if prefixErr != nil && !errors.Is(prefixErr, io.EOF) {
		return imageMetadata{}, errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("read image signature: %w", prefixErr))
	}
	if unsupported, classifyErr := unsupportedFormat(ctx, file, prefix[:prefixSize], size); classifyErr != nil {
		return imageMetadata{}, classifyErr
	} else if unsupported {
		return imageMetadata{}, ErrUnsupportedMediaType
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return imageMetadata{}, errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("rewind image signature: %w", err))
	}
	config, format, err := image.DecodeConfig(contextFileReader{ctx: ctx, file: file})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return imageMetadata{}, ctxErr
	}
	if err != nil {
		return imageMetadata{}, errors.Join(ErrInvalidMedia, err)
	}
	contentType := imageContentType(format)
	if contentType == "" {
		return imageMetadata{}, ErrUnsupportedMediaType
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > MaxImageWidth || config.Height > MaxImageHeight || int64(config.Width)*int64(config.Height) > MaxImageArea {
		return imageMetadata{}, ErrMediaDimensions
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return imageMetadata{}, errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("rewind image for decode: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return imageMetadata{}, err
	}
	select {
	case decodeSlot <- struct{}{}:
	case <-ctx.Done():
		return imageMetadata{}, ctx.Err()
	}
	defer func() { <-decodeSlot }()
	if err := ctx.Err(); err != nil {
		return imageMetadata{}, err
	}
	_, decodedFormat, decodeErr := image.Decode(contextFileReader{ctx: ctx, file: file})
	if ctxErr := ctx.Err(); ctxErr != nil {
		return imageMetadata{}, ctxErr
	}
	if decodeErr != nil {
		return imageMetadata{}, errors.Join(ErrInvalidMedia, decodeErr)
	}
	if imageContentType(decodedFormat) != contentType {
		return imageMetadata{}, ErrInvalidMedia
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return imageMetadata{}, errors.Join(ErrMediaStorageUnavailable, fmt.Errorf("rewind image after decode: %w", err))
	}
	return imageMetadata{ContentType: contentType, ByteSize: size, Width: config.Width, Height: config.Height}, nil
}

func unsupportedFormat(ctx context.Context, file *os.File, prefix []byte, size int64) (bool, error) {
	trimmed := bytes.TrimSpace(bytes.TrimPrefix(prefix, []byte{0xef, 0xbb, 0xbf}))
	if len(trimmed) > 0 && trimmed[0] == '<' {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return false, errors.Join(ErrMediaStorageUnavailable, err)
		}
		reader := contextFileReader{ctx: ctx, file: file}
		decoder := xml.NewDecoder(reader)
		seenRoot, svg := false, false
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				return seenRoot && svg, nil
			}
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return false, ctxErr
				}
				return false, errors.Join(ErrInvalidMedia, err)
			}
			if start, ok := token.(xml.StartElement); ok && !seenRoot {
				seenRoot = true
				svg = strings.EqualFold(start.Name.Local, "svg")
			}
		}
	}
	if len(prefix) >= 20 && string(prefix[:4]) == "RIFF" && string(prefix[8:12]) == "WEBP" {
		riffSize, chunkSize := binary.LittleEndian.Uint32(prefix[4:8]), binary.LittleEndian.Uint32(prefix[16:20])
		if riffSize >= 12 && int64(riffSize)+8 <= size && int64(chunkSize)+20 <= size {
			return true, nil
		}
	}
	if len(prefix) >= 26 && string(prefix[:2]) == "BM" {
		headerSize := binary.LittleEndian.Uint32(prefix[2:6])
		offset := binary.LittleEndian.Uint32(prefix[10:14])
		dibSize := binary.LittleEndian.Uint32(prefix[14:18])
		if headerSize >= 26 && int64(headerSize) <= size && offset >= 26 && int64(offset) <= size && dibSize >= 12 {
			return true, nil
		}
	}
	if len(prefix) >= 8 && (string(prefix[:4]) == "II\x2a\x00" || string(prefix[:4]) == "MM\x00\x2a") {
		var offset uint32
		if prefix[0] == 'I' {
			offset = binary.LittleEndian.Uint32(prefix[4:8])
		} else {
			offset = binary.BigEndian.Uint32(prefix[4:8])
		}
		if offset >= 8 && int64(offset) < size {
			return true, nil
		}
	}
	if len(prefix) >= 12 && string(prefix[4:8]) == "ftyp" {
		brand := string(prefix[8:12])
		if brand == "heic" || brand == "heix" || brand == "hevc" || brand == "hevx" || brand == "avif" || brand == "avis" {
			return true, nil
		}
	}
	return false, nil
}

func imageContentType(format string) string {
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	default:
		return ""
	}
}

func sameMetadata(upload Upload, metadata imageMetadata) bool {
	return upload.ContentType == metadata.ContentType && upload.ByteSize == metadata.ByteSize && upload.Width == metadata.Width && upload.Height == metadata.Height && strings.EqualFold(upload.SHA256, metadata.SHA256)
}
