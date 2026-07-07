package note

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNormalizeListInputTrimsCategorySlug(t *testing.T) {
	normalized := NormalizeListInput(ListInput{
		CategorySlug: " food ",
		Limit:        12,
	})

	want := ListInput{
		CategorySlug: "food",
		Limit:        12,
	}
	if diff := cmp.Diff(want, normalized); diff != "" {
		t.Fatalf("normalized input mismatch (-want +got):\n%s", diff)
	}
}

func TestNormalizeListInputUsesDefaultLimit(t *testing.T) {
	normalized := NormalizeListInput(ListInput{
		CategorySlug: "food",
	})

	if normalized.Limit != ListDefaultLimit {
		t.Fatalf("limit = %d, want %d", normalized.Limit, ListDefaultLimit)
	}
}

func TestValidateListInputRejectsNegativeLimit(t *testing.T) {
	problems := ValidateListInput(ListInput{
		CategorySlug: "food",
		Limit:        -1,
	})

	want := []ValidationProblem{
		{Field: "limit", Message: "required"},
	}
	if diff := cmp.Diff(want, problems); diff != "" {
		t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
	}
}
