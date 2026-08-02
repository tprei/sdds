package note

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	// EmbeddingModelID is the Hugging Face repository for the production
	// Portuguese sentence-embedding model. It is pinned to EmbeddingModelRevision.
	EmbeddingModelID = "PORTULAN/serafim-100m-portuguese-pt-sentence-encoder-ir"
	// EmbeddingModelRevision is the immutable git revision the production model
	// is exported from. Production startup never downloads an unpinned revision.
	EmbeddingModelRevision = "f27c45d197ea6541dd071b1d992ec91776ee76bd"
	// EmbeddingDimension is the fixed width of every passage and query vector.
	// A stored vector whose length disagrees with this is rejected on read.
	EmbeddingDimension = 768
)

// Embedding is a normalized passage vector plus the provenance needed to decide
// whether it is still current. The Vector is unit-length (L2-normalized) so that
// cosine similarity is a plain dot product.
type Embedding struct {
	ModelID       string
	ModelRevision string
	Dimension     int
	SourceSHA256  string
	Vector        []float32
}

// EmbeddingPassage builds the deterministic text that is embedded for a note.
// The title leads the passage so the most identifying text survives the model's
// 128-token truncation window.
func EmbeddingPassage(title, body string) string {
	return strings.TrimSpace(title) + "\n\n" + strings.TrimSpace(body)
}

// EmbeddingFingerprint is the lowercase hex SHA-256 of a passage. It lets the
// reindex command skip notes whose stored passage has not changed without
// re-embedding them.
func EmbeddingFingerprint(passage string) string {
	sum := sha256.Sum256([]byte(passage))
	return hex.EncodeToString(sum[:])
}
