package note

// SearchVersion identifies the retrieval implementation that produced results.
type SearchVersion string

const CurrentSearchVersion SearchVersion = "fts5-v1"

// RetrievalSource identifies the candidate source for a search result.
type RetrievalSource string

const CurrentRetrievalSource RetrievalSource = "lexical"
