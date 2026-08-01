package sqlite

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/note"
)

func TestEncodeVectorRoundTripsProductionDimension(t *testing.T) {
	pattern := []float32{0, 1, -1, 0.5, -0.25, math.MaxFloat32, math.SmallestNonzeroFloat32, 3.14159}
	values := make([]float32, note.EmbeddingDimension)
	for index := range values {
		values[index] = pattern[index%len(pattern)]
	}
	blob := encodeVector(values)
	if len(blob) != note.EmbeddingDimension*4 {
		t.Fatalf("blob length = %d, want %d", len(blob), note.EmbeddingDimension*4)
	}

	decoded, err := decodeVector(blob, note.EmbeddingDimension)
	if err != nil {
		t.Fatalf("decode vector: %v", err)
	}
	if diff := cmp.Diff(values, decoded); diff != "" {
		t.Fatalf("round trip mismatch (-want +got):\n%s", diff)
	}
}

func TestEncodeVectorOfEmptySliceYieldsEmptyBlob(t *testing.T) {
	if blob := encodeVector(nil); len(blob) != 0 {
		t.Fatalf("nil slice blob length = %d, want 0", len(blob))
	}
	if blob := encodeVector([]float32{}); len(blob) != 0 {
		t.Fatalf("empty slice blob length = %d, want 0", len(blob))
	}
}

func TestDecodeVectorRejectsShortBlob(t *testing.T) {
	blob := make([]byte, note.EmbeddingDimension*4-4)
	if _, err := decodeVector(blob, note.EmbeddingDimension); err == nil {
		t.Fatal("expected dimension mismatch error for short blob, got nil")
	}
}

func TestDecodeVectorRejectsLongBlob(t *testing.T) {
	blob := make([]byte, note.EmbeddingDimension*4+4)
	if _, err := decodeVector(blob, note.EmbeddingDimension); err == nil {
		t.Fatal("expected dimension mismatch error for long blob, got nil")
	}
}

func TestDecodeVectorRejectsWrongDeclaredDimension(t *testing.T) {
	blob := make([]byte, 4)
	if _, err := decodeVector(blob, 1); err == nil {
		t.Fatal("expected dimension mismatch error for declared dimension 1, got nil")
	}
}
