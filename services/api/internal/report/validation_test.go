package report

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

const testReporterUserID = "018ff5b8-0000-7000-8000-000000000201"

func TestValidateCreateInputReportsEachPublicField(t *testing.T) {
	tests := []struct {
		name  string
		input CreateInput
		want  []ValidationProblem
	}{
		{
			name:  "missing target type",
			input: CreateInput{TargetID: "n1", Reason: ReasonSpam, ReporterUserID: testReporterUserID},
			want:  []ValidationProblem{{Field: "target_type", Code: "required"}},
		},
		{
			name:  "unknown target type",
			input: CreateInput{TargetType: "account", TargetID: "n1", Reason: ReasonSpam, ReporterUserID: testReporterUserID},
			want:  []ValidationProblem{{Field: "target_type", Code: "invalid"}},
		},
		{
			name:  "missing target id",
			input: CreateInput{TargetType: TargetTypeNote, Reason: ReasonSpam, ReporterUserID: testReporterUserID},
			want:  []ValidationProblem{{Field: "target_id", Code: "required"}},
		},
		{
			name:  "whitespace target id is missing",
			input: CreateInput{TargetType: TargetTypeNote, TargetID: "  \t\n ", Reason: ReasonSpam, ReporterUserID: testReporterUserID},
			want:  []ValidationProblem{{Field: "target_id", Code: "required"}},
		},
		{
			name:  "missing reason",
			input: CreateInput{TargetType: TargetTypeComment, TargetID: "c1", ReporterUserID: testReporterUserID},
			want:  []ValidationProblem{{Field: "reason", Code: "required"}},
		},
		{
			name:  "unknown reason",
			input: CreateInput{TargetType: TargetTypeNote, TargetID: "n1", Reason: "undisclosed_commercial", ReporterUserID: testReporterUserID},
			want:  []ValidationProblem{{Field: "reason", Code: "invalid"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ValidateCreateInput(test.input)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Fatalf("problems mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateCreateInputAcceptsAllFourReasons(t *testing.T) {
	for _, reason := range []Reason{ReasonSpam, ReasonHarassment, ReasonHarmfulOrMisleading, ReasonOther} {
		input := CreateInput{TargetType: TargetTypeNote, TargetID: "n1", Reason: reason, ReporterUserID: testReporterUserID}
		if problems := ValidateCreateInput(input); len(problems) != 0 {
			t.Fatalf("reason %q problems = %v, want none", reason, problems)
		}
	}
}

func TestValidateCreateInputAcceptsBothTargetTypes(t *testing.T) {
	for _, target := range []TargetType{TargetTypeNote, TargetTypeComment} {
		input := CreateInput{TargetType: target, TargetID: "t1", Reason: ReasonSpam, ReporterUserID: testReporterUserID}
		if problems := ValidateCreateInput(input); len(problems) != 0 {
			t.Fatalf("target type %q problems = %v, want none", target, problems)
		}
	}
}

func TestValidateCreateInputDetailsLengthBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		details      string
		wantProblem  bool
		wantNilStore bool
	}{
		{name: "empty becomes nil", details: "", wantNilStore: true},
		{name: "whitespace becomes nil", details: "  \t\n ", wantNilStore: true},
		{name: "exactly 1000 code points", details: strings.Repeat("é", DetailsMaxLength)},
		{name: "1001 code points too long", details: strings.Repeat("é", DetailsMaxLength+1), wantProblem: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			details := test.details
			input := CreateInput{TargetType: TargetTypeNote, TargetID: "n1", Reason: ReasonSpam, Details: &details, ReporterUserID: testReporterUserID}
			problems := ValidateCreateInput(input)
			if test.wantProblem {
				if diff := cmp.Diff([]ValidationProblem{{Field: "details", Code: "too_long"}}, problems); diff != "" {
					t.Fatalf("problems mismatch (-want +got):\n%s", diff)
				}
				return
			}
			if len(problems) != 0 {
				t.Fatalf("problems = %v, want none", problems)
			}
		})
	}
}

func TestNormalizeCreateInputTrimsAndNilsDetails(t *testing.T) {
	details := "  conta o que aconteceu \t"
	normalized := NormalizeCreateInput(CreateInput{
		TargetType:     TargetTypeNote,
		TargetID:       "  n1\n",
		Reason:         ReasonSpam,
		Details:        &details,
		ReporterUserID: testReporterUserID,
	})
	if normalized.TargetID != "n1" {
		t.Fatalf("target id = %q, want %q", normalized.TargetID, "n1")
	}
	if normalized.Details == nil || *normalized.Details != "conta o que aconteceu" {
		t.Fatalf("details = %v, want %q", normalized.Details, "conta o que aconteceu")
	}

	blank := "   "
	normalizedBlank := NormalizeCreateInput(CreateInput{
		TargetType:     TargetTypeNote,
		TargetID:       "n1",
		Reason:         ReasonSpam,
		Details:        &blank,
		ReporterUserID: testReporterUserID,
	})
	if normalizedBlank.Details != nil {
		t.Fatalf("blank details = %v, want nil", normalizedBlank.Details)
	}

	empty := NormalizeCreateInput(CreateInput{TargetType: TargetTypeNote, TargetID: "n1", Reason: ReasonSpam, ReporterUserID: testReporterUserID})
	if empty.Details != nil {
		t.Fatalf("absent details = %v, want nil", empty.Details)
	}
}

func TestValidateCreateInputNeverExposesReporterField(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{TargetType: TargetTypeNote, TargetID: "n1", Reason: ReasonSpam, ReporterUserID: ""})
	for _, problem := range problems {
		if problem.Field == "reporter_user_id" {
			t.Fatalf("reporter_user_id leaked as validation field: %+v", problem)
		}
	}
}

func TestNewIDReturnsUUIDv7(t *testing.T) {
	ids := make(map[ID]struct{}, 2)
	for range 2 {
		id, err := NewID()
		if err != nil {
			t.Fatalf("create id: %v", err)
		}
		parsed, err := uuid.Parse(string(id))
		if err != nil {
			t.Fatalf("parse id %q: %v", id, err)
		}
		if parsed.Version() != 7 {
			t.Fatalf("id version = %d, want 7", parsed.Version())
		}
		ids[id] = struct{}{}
	}
	if len(ids) != 2 {
		t.Fatal("NewID produced duplicate ids")
	}
}

func TestValidateInternalRejectsMissingReporter(t *testing.T) {
	if err := ValidateInternal(CreateInput{TargetType: TargetTypeNote, TargetID: "n1", Reason: ReasonSpam, ReporterUserID: ""}); !errors.Is(err, ErrMissingReporter) {
		t.Fatalf("missing reporter error = %v, want ErrMissingReporter", err)
	}
	if err := ValidateInternal(CreateInput{TargetType: TargetTypeNote, TargetID: "n1", Reason: ReasonSpam, ReporterUserID: testReporterUserID}); err != nil {
		t.Fatalf("present reporter error = %v, want nil", err)
	}
}

func TestValidateCreateInputEmitsNoProblemForMissingReporter(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{TargetType: TargetTypeNote, TargetID: "n1", Reason: ReasonSpam, ReporterUserID: ""})
	if len(problems) != 0 {
		t.Fatalf("public problems = %v, want none for missing reporter", problems)
	}
	for _, problem := range problems {
		if problem.Field == "reporter_user_id" {
			t.Fatalf("reporter_user_id leaked as validation field: %+v", problem)
		}
	}
}
