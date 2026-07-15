package note

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValidateCreateInputAcceptsCategoryAndOptionalPlace(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{
		Title:           "Café bom",
		Body:            "Tem pão de queijo decente.",
		ClientRequestID: "domain-valid",
		CategorySlug:    "food",
	})

	if len(problems) != 0 {
		t.Fatalf("problem count = %d, want 0", len(problems))
	}
}

func TestValidateCreateInputAllowsUnknownCatalogMetadata(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{
		Title:           "Café bom",
		Body:            "Tem pão de queijo decente.",
		CategorySlug:    "qualquer",
		ClientRequestID: "domain-unknown-catalog",
		PlaceSlug:       "qualquer-lugar",
	})

	if len(problems) != 0 {
		t.Fatalf("problem count = %d, want 0", len(problems))
	}
}

func TestNormalizeCreateInputTrimsBoundaryFields(t *testing.T) {
	normalized := NormalizeCreateInput(CreateInput{
		Title:        " Café bom ",
		Body:         "\nTem pão de queijo.\n",
		CategorySlug: " food ",
		PlaceSlug:    " sao-paulo ",
	})

	want := CreateInput{
		Title:        "Café bom",
		Body:         "Tem pão de queijo.",
		CategorySlug: "food",
		PlaceSlug:    "sao-paulo",
	}
	if diff := cmp.Diff(want, normalized); diff != "" {
		t.Fatalf("normalized input mismatch (-want +got):\n%s", diff)
	}
}

func TestNormalizeAuthorNotesInputDefaultsLimit(t *testing.T) {
	input := NormalizeAuthorNotesInput(AuthorNotesInput{})

	if input.Limit != AuthorNotesDefaultLimit {
		t.Fatalf("limit = %d, want %d", input.Limit, AuthorNotesDefaultLimit)
	}
}

func TestValidateCreateInputTreatsTrimmedEmptyCategoryAsRequired(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{
		Title:           "Café bom",
		Body:            "Tem pão de queijo decente.",
		ClientRequestID: "domain-invalid-category",
		CategorySlug:    "   ",
		PlaceSlug:       "\n\t",
	})

	want := []ValidationProblem{
		{Field: "category_slug", Message: "required"},
	}
	if diff := cmp.Diff(want, problems); diff != "" {
		t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
	}
}
