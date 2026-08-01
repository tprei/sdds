package note

// SemanticCandidateLimit is the bounded candidate list size each retrieval
// source (lexical, semantic) contributes before fusion. It is not the final
// response size -- see SearchDefaultLimit for that.
const SemanticCandidateLimit = 100

// SemanticSearchInput carries the already-embedded query vector, the bounded
// candidate limit, and the category constraint. It carries no SQL, no store
// type, and no model detail -- the store implementation owns all of that.
type SemanticSearchInput struct {
	Vector       []float32
	CategorySlug CategorySlug
	Limit        int
}

// ScoredNote carries note identity and its cosine similarity score. Fusion
// consumes rank, not score, so the score never reaches the API surface.
type ScoredNote struct {
	NoteID string
	Score  float32
}
