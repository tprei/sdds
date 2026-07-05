package note

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNormalizeSearchInputTrimsQuery(t *testing.T) {
	normalized := NormalizeSearchInput(SearchInput{
		Query: "  café bom  ",
		Limit: 12,
	})

	want := SearchInput{
		Query: "café bom",
		Limit: 12,
	}
	if diff := cmp.Diff(want, normalized); diff != "" {
		t.Fatalf("normalized input mismatch (-want +got):\n%s", diff)
	}
}

func TestNormalizeSearchInputUsesDefaultLimit(t *testing.T) {
	normalized := NormalizeSearchInput(SearchInput{
		Query: "café bom",
	})

	if normalized.Limit != SearchDefaultLimit {
		t.Fatalf("limit = %d, want %d", normalized.Limit, SearchDefaultLimit)
	}
}

func TestValidateSearchInputAcceptsQueryAndLimit(t *testing.T) {
	problems := ValidateSearchInput(SearchInput{
		Query: "café bom",
		Limit: 12,
	})

	if len(problems) != 0 {
		t.Fatalf("problem count = %d, want 0", len(problems))
	}
}

func TestValidateSearchInputTreatsTrimmedEmptyQueryAsRequired(t *testing.T) {
	problems := ValidateSearchInput(SearchInput{
		Query: " \n\t ",
		Limit: 12,
	})

	want := []ValidationProblem{
		{Field: "q", Message: "required"},
	}
	if diff := cmp.Diff(want, problems); diff != "" {
		t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateSearchInputRejectsLongQuery(t *testing.T) {
	problems := ValidateSearchInput(SearchInput{
		Query: strings.Repeat("a", SearchQueryMaxLength+1),
		Limit: 12,
	})

	want := []ValidationProblem{
		{Field: "q", Message: "too_long"},
	}
	if diff := cmp.Diff(want, problems); diff != "" {
		t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateSearchInputCountsRunes(t *testing.T) {
	problems := ValidateSearchInput(SearchInput{
		Query: strings.Repeat("ç", SearchQueryMaxLength),
		Limit: 12,
	})

	if len(problems) != 0 {
		t.Fatalf("problem count = %d, want 0", len(problems))
	}
}

func TestValidateSearchInputRejectsNegativeLimit(t *testing.T) {
	problems := ValidateSearchInput(SearchInput{
		Query: "café bom",
		Limit: -1,
	})

	want := []ValidationProblem{
		{Field: "limit", Message: "required"},
	}
	if diff := cmp.Diff(want, problems); diff != "" {
		t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
	}
}
