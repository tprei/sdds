package note

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFuseSearchCandidatesRanksNoteInBothListsAboveNoteInOne(t *testing.T) {
	// "both" appears at rank 2 in each list; "lexical-only" appears at rank 1
	// in the lexical list alone. Rank-1-in-one-list beats rank-2-in-both
	// under RRF unless both lists agree, so place "both" at rank 1 in each
	// to make the comparison unambiguous.
	fused := FuseSearchCandidates(
		[]string{"both", "lexical-only"},
		[]string{"both", "semantic-only"},
		10,
	)
	if len(fused) != 3 {
		t.Fatalf("fused count = %d, want 3", len(fused))
	}
	if fused[0].NoteID != "both" {
		t.Fatalf("fused[0] = %q, want %q (present in both lists)", fused[0].NoteID, "both")
	}
	if fused[0].RetrievalSource != RetrievalSourceHybrid {
		t.Fatalf("fused[0] source = %q, want hybrid", fused[0].RetrievalSource)
	}
}

func TestFuseSearchCandidatesLabelsRetrievalSourceCorrectly(t *testing.T) {
	fused := FuseSearchCandidates([]string{"lex", "both"}, []string{"sem", "both"}, 10)
	sources := map[string]RetrievalSource{}
	for _, candidate := range fused {
		sources[candidate.NoteID] = candidate.RetrievalSource
	}
	if sources["lex"] != RetrievalSourceLexical {
		t.Fatalf("lex source = %q, want lexical", sources["lex"])
	}
	if sources["sem"] != RetrievalSourceSemantic {
		t.Fatalf("sem source = %q, want semantic", sources["sem"])
	}
	if sources["both"] != RetrievalSourceHybrid {
		t.Fatalf("both source = %q, want hybrid", sources["both"])
	}
}

func TestFuseSearchCandidatesIsDeterministicAcrossRepeatedCalls(t *testing.T) {
	lexical := []string{"n1", "n2", "n3", "n4", "n5"}
	semantic := []string{"n5", "n4", "n6", "n7", "n1"}

	first := FuseSearchCandidates(lexical, semantic, 10)
	for i := 0; i < 100; i++ {
		got := FuseSearchCandidates(lexical, semantic, 10)
		if diff := cmp.Diff(first, got); diff != "" {
			t.Fatalf("run %d diverged from run 0 (-want +got):\n%s", i, diff)
		}
	}
}

func TestFuseSearchCandidatesTruncatesToLimit(t *testing.T) {
	lexical := []string{"n1", "n2", "n3", "n4", "n5"}
	fused := FuseSearchCandidates(lexical, nil, 2)
	if len(fused) != 2 {
		t.Fatalf("fused count = %d, want 2", len(fused))
	}
}

func TestFuseSearchCandidatesReturnsSemanticOnlyWhenLexicalEmpty(t *testing.T) {
	fused := FuseSearchCandidates(nil, []string{"s1", "s2"}, 10)
	if len(fused) != 2 {
		t.Fatalf("fused count = %d, want 2", len(fused))
	}
	for _, candidate := range fused {
		if candidate.RetrievalSource != RetrievalSourceSemantic {
			t.Fatalf("candidate %q source = %q, want semantic", candidate.NoteID, candidate.RetrievalSource)
		}
	}
}

func TestFuseSearchCandidatesReturnsLexicalOnlyWhenSemanticEmpty(t *testing.T) {
	fused := FuseSearchCandidates([]string{"l1", "l2"}, nil, 10)
	if len(fused) != 2 {
		t.Fatalf("fused count = %d, want 2", len(fused))
	}
	for _, candidate := range fused {
		if candidate.RetrievalSource != RetrievalSourceLexical {
			t.Fatalf("candidate %q source = %q, want lexical", candidate.NoteID, candidate.RetrievalSource)
		}
	}
}

func TestFuseSearchCandidatesBreaksTiesByNoteIDDescending(t *testing.T) {
	// Two notes at the same rank in disjoint lists have equal RRF scores;
	// the tie-break must be deterministic (note id descending), not
	// dependent on map iteration order.
	fused := FuseSearchCandidates([]string{"aaa"}, []string{"zzz"}, 10)
	if len(fused) != 2 {
		t.Fatalf("fused count = %d, want 2", len(fused))
	}
	if fused[0].NoteID != "zzz" || fused[1].NoteID != "aaa" {
		t.Fatalf("fused order = [%q, %q], want [zzz, aaa] (descending id tie-break)", fused[0].NoteID, fused[1].NoteID)
	}
}
