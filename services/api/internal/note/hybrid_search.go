package note

import "sort"

// ReciprocalRankFusionK is the RRF smoothing constant: a note's contribution
// from one list is 1/(ReciprocalRankFusionK + rank), rank 1-based.
const ReciprocalRankFusionK = 60

// SearchResult is one hydrated, ranked search result.
type SearchResult struct {
	Note            Note
	RetrievalSource RetrievalSource
}

// FusedCandidate is one note id ranked by fusion, with no note payload yet --
// hydration happens above this layer.
type FusedCandidate struct {
	NoteID          string
	RetrievalSource RetrievalSource
}

type fusionEntry struct {
	noteID          string
	score           float64
	bestRank        int
	retrievalSource RetrievalSource
}

// FuseSearchCandidates merges two ranked note-id lists with deterministic
// reciprocal-rank fusion. It knows nothing about how either list was
// produced.
//
// Ordering is score descending, then best rank ascending, then note id
// descending. UUIDv7 ids sort by creation time, so this surfaces the newest
// note first on a tie. The result is a strict total order over unique note
// ids, so it never depends on map iteration order.
func FuseSearchCandidates(lexical, semantic []string, limit int) []FusedCandidate {
	entries := make(map[string]*fusionEntry)

	// accumulate adds one ranked source's RRF contributions and provenance
	// to the shared entry map.
	accumulate := func(ids []string, source RetrievalSource) {
		for index, id := range ids {
			rank := index + 1
			entry, ok := entries[id]
			if !ok {
				entry = &fusionEntry{noteID: id, bestRank: rank, retrievalSource: source}
				entries[id] = entry
			} else if entry.retrievalSource != source {
				entry.retrievalSource = RetrievalSourceHybrid
			}
			entry.score += 1 / float64(ReciprocalRankFusionK+rank)
			if rank < entry.bestRank {
				entry.bestRank = rank
			}
		}
	}
	accumulate(lexical, RetrievalSourceLexical)
	accumulate(semantic, RetrievalSourceSemantic)

	fused := make([]*fusionEntry, 0, len(entries))
	for _, entry := range entries {
		fused = append(fused, entry)
	}
	sort.Slice(fused, func(i, j int) bool {
		if fused[i].score != fused[j].score {
			return fused[i].score > fused[j].score
		}
		if fused[i].bestRank != fused[j].bestRank {
			return fused[i].bestRank < fused[j].bestRank
		}
		return fused[i].noteID > fused[j].noteID
	})
	if limit >= 0 && len(fused) > limit {
		fused = fused[:limit]
	}

	candidates := make([]FusedCandidate, len(fused))
	for i, entry := range fused {
		candidates[i] = FusedCandidate{NoteID: entry.noteID, RetrievalSource: entry.retrievalSource}
	}
	return candidates
}
