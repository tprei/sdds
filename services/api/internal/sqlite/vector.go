package sqlite

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/tprei/sdds/services/api/internal/note"
)

// ErrVectorDimensionMismatch is returned when a stored vector blob does not
// match the expected note.EmbeddingDimension, either because the blob length
// disagrees with its declared dimension column or because the declared
// dimension is not the production value. A truncated vector is never returned.
var ErrVectorDimensionMismatch = errors.New("vector dimension mismatch")

// encodeVector serializes a float32 vector as a little-endian BLOB. An empty
// slice yields an empty BLOB, which the note_embeddings CHECK constraint
// (length(vector) = dimension * 4) rejects at insert; callers that store an
// embedding must validate its length first.
func encodeVector(values []float32) []byte {
	blob := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(blob[index*4:], math.Float32bits(value))
	}
	return blob
}

// decodeVector reads a little-endian float32 BLOB produced by encodeVector. The
// declared dimension is the value stored alongside the vector; it must equal
// note.EmbeddingDimension and must agree with the blob length, otherwise the
// row is treated as corruption and ErrVectorDimensionMismatch is returned.
func decodeVector(blob []byte, dimension int) ([]float32, error) {
	if dimension != note.EmbeddingDimension {
		return nil, fmt.Errorf("%w: declared dimension %d, want %d",
			ErrVectorDimensionMismatch, dimension, note.EmbeddingDimension)
	}
	if len(blob) != dimension*4 {
		return nil, fmt.Errorf("%w: blob length %d, want %d",
			ErrVectorDimensionMismatch, len(blob), dimension*4)
	}
	values := make([]float32, dimension)
	for index := range values {
		bits := binary.LittleEndian.Uint32(blob[index*4:])
		values[index] = math.Float32frombits(bits)
	}
	return values, nil
}
