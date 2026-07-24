package comment

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tprei/sdds/services/api/internal/user"
)

func TestNormalizeCreateInput(t *testing.T) {
	input := CreateInput{
		NoteID: "note-id",
		UserID: user.UserID("user-id"),
		Body:   " \n comentário com emoji 😀 \t",
	}

	got := NormalizeCreateInput(input)
	want := CreateInput{
		NoteID: "note-id",
		UserID: user.UserID("user-id"),
		Body:   "comentário com emoji 😀",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("normalized create input mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateCreateInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []ValidationProblem
	}{
		{
			name: "accepts trimmed body",
			body: "\n comentário \t",
			want: []ValidationProblem{},
		},
		{
			name: "rejects empty body",
			want: []ValidationProblem{{Field: "body", Code: "required"}},
		},
		{
			name: "rejects whitespace-only body",
			body: " \t\n ",
			want: []ValidationProblem{{Field: "body", Code: "required"}},
		},
		{
			name: "accepts 1000 unicode code points",
			body: strings.Repeat("😀", BodyMaxLength),
			want: []ValidationProblem{},
		},
		{
			name: "rejects 1001 unicode code points",
			body: strings.Repeat("😀", BodyMaxLength+1),
			want: []ValidationProblem{{Field: "body", Code: "too_long"}},
		},
		{
			name: "counts zwj components as code points",
			body: strings.Repeat("👩‍💻", 334),
			want: []ValidationProblem{{Field: "body", Code: "too_long"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ValidateCreateInput(CreateInput{Body: test.body})
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNormalizeListInput(t *testing.T) {
	position := &Position{PageKey: 42}
	input := ListInput{NoteID: "note-id", After: position}

	got := NormalizeListInput(input)
	want := ListInput{
		NoteID: "note-id",
		Limit:  ListDefaultLimit,
		After:  position,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("normalized list input mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateListInput(t *testing.T) {
	tests := []struct {
		name  string
		input ListInput
		want  []ValidationProblem
	}{
		{
			name: "accepts default limit",
			want: []ValidationProblem{},
		},
		{
			name:  "accepts minimum limit",
			input: ListInput{Limit: 1},
			want:  []ValidationProblem{},
		},
		{
			name:  "accepts maximum limit",
			input: ListInput{Limit: ListMaxLimit},
			want:  []ValidationProblem{},
		},
		{
			name:  "rejects negative limit",
			input: ListInput{Limit: -1},
			want:  []ValidationProblem{{Field: "limit", Code: "invalid"}},
		},
		{
			name:  "rejects limit above maximum",
			input: ListInput{Limit: ListMaxLimit + 1},
			want:  []ValidationProblem{{Field: "limit", Code: "invalid"}},
		},
		{
			name:  "rejects zero page key",
			input: ListInput{After: &Position{}},
			want:  []ValidationProblem{{Field: "cursor", Code: "invalid"}},
		},
		{
			name:  "rejects negative page key",
			input: ListInput{After: &Position{PageKey: -1}},
			want:  []ValidationProblem{{Field: "cursor", Code: "invalid"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ValidateListInput(test.input)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
