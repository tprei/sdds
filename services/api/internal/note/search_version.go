package note

// SearchVersion identifies the retrieval implementation that produced results.
type SearchVersion string

// CurrentSearchVersion is the explicit, versioned identifier for the hybrid
// lexical + semantic retrieval path. There is no runtime fallback search
// implementation, so this is the only search version the API ever emits.
const CurrentSearchVersion SearchVersion = "hybrid-serafim100m-fts5-v1"

// RetrievalSource identifies which retrieval source(s) contributed a search
// result: only the lexical (FTS5) candidate list, only the semantic (cosine
// KNN) candidate list, or both.
type RetrievalSource string

const (
	RetrievalSourceLexical  RetrievalSource = "lexical"
	RetrievalSourceSemantic RetrievalSource = "semantic"
	RetrievalSourceHybrid   RetrievalSource = "hybrid"
)
