package note

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

var ErrIdempotencyConflict = errors.New("note create request conflict")

func CreateRequestFingerprint(input CreateInput) string {
	input = NormalizeCreateInput(input)

	hasher := sha256.New()
	var length [8]byte
	writeFrame := func(value []byte) {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hasher.Write(length[:])
		_, _ = hasher.Write(value)
	}
	writeUint64 := func(value uint64) {
		binary.BigEndian.PutUint64(length[:], value)
		_, _ = hasher.Write(length[:])
	}

	writeFrame([]byte("note.create.v1"))
	writeFrame([]byte(input.Title))
	writeFrame([]byte(input.Body))
	writeFrame([]byte(input.CategorySlug))
	if input.PlaceSlug == "" {
		_, _ = hasher.Write([]byte{0})
	} else {
		_, _ = hasher.Write([]byte{1})
		writeFrame([]byte(input.PlaceSlug))
	}
	writeUint64(uint64(len(input.ImageUploadIDs)))
	for _, imageUploadID := range input.ImageUploadIDs {
		writeFrame([]byte(imageUploadID))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
