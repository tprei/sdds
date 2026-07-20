package note

import (
	"strings"
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

func TestValidateCreateInputClientRequestIDAndImages(t *testing.T) {
	validInput := func() CreateInput {
		return CreateInput{
			Title:           "Café bom",
			Body:            "Funciona.",
			CategorySlug:    "food",
			ClientRequestID: "client-request",
		}
	}
	tests := []struct {
		name   string
		mutate func(*CreateInput)
		want   []ValidationProblem
	}{
		{
			name: "client request id empty",
			mutate: func(input *CreateInput) {
				input.ClientRequestID = ""
			},
			want: []ValidationProblem{{Field: "client_request_id", Message: "required"}},
		},
		{
			name: "client request id max length",
			mutate: func(input *CreateInput) {
				input.ClientRequestID = strings.Repeat("a", ClientRequestIDMaxLength)
			},
			want: []ValidationProblem{},
		},
		{
			name: "client request id over max length",
			mutate: func(input *CreateInput) {
				input.ClientRequestID = strings.Repeat("a", ClientRequestIDMaxLength+1)
			},
			want: []ValidationProblem{{Field: "client_request_id", Message: "too_long"}},
		},
		{name: "zero images", want: []ValidationProblem{}},
		{
			name: "one image",
			mutate: func(input *CreateInput) {
				input.ImageUploadIDs = []string{"upload-1"}
			},
			want: []ValidationProblem{},
		},
		{
			name: "two images",
			mutate: func(input *CreateInput) {
				input.ImageUploadIDs = []string{"upload-1", "upload-2"}
			},
			want: []ValidationProblem{{Field: "image_upload_ids", Message: "too_long"}},
		},
		{
			name: "empty image id",
			mutate: func(input *CreateInput) {
				input.ImageUploadIDs = []string{""}
			},
			want: []ValidationProblem{{Field: "image_upload_ids", Message: "invalid"}},
		},
		{
			name: "duplicate image id",
			mutate: func(input *CreateInput) {
				input.ImageUploadIDs = []string{"upload-1", "upload-1"}
			},
			want: []ValidationProblem{
				{Field: "image_upload_ids", Message: "too_long"},
				{Field: "image_upload_ids", Message: "invalid"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validInput()
			if tt.mutate != nil {
				tt.mutate(&input)
			}
			if diff := cmp.Diff(tt.want, ValidateCreateInput(input)); diff != "" {
				t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
