package user

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNormalizeCreateUserInput(t *testing.T) {
	got := NormalizeCreateUserInput(CreateUserInput{
		Username:    "  Thiago.Dev  ",
		Password:    "  secret-password  ",
		DisplayName: "  Thiago Dev  ",
	})

	want := CreateUserInput{
		Username:    "thiago.dev",
		Password:    "  secret-password  ",
		DisplayName: "Thiago Dev",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("normalized input mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateCreateUserInputAcceptsValidInput(t *testing.T) {
	problems := ValidateCreateUserInput(CreateUserInput{
		Username:    "thiago-01",
		Password:    "secret-password",
		DisplayName: "Thiago",
	})
	if len(problems) != 0 {
		t.Fatalf("problems = %#v, want none", problems)
	}
}

func TestValidateCreateUserInputReportsInvalidFields(t *testing.T) {
	got := ValidateCreateUserInput(CreateUserInput{
		Username:    "té",
		Password:    "short",
		DisplayName: strings.Repeat("a", DisplayNameMaxLength+1),
	})
	want := []ValidationProblem{
		{Field: "username", Code: "too_short"},
		{Field: "password", Code: "too_short"},
		{Field: "display_name", Code: "too_long"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("problems mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateCreateUserInputRejectsUnsupportedUsernameCharacters(t *testing.T) {
	got := ValidateCreateUserInput(CreateUserInput{
		Username:    "thiago@dev",
		Password:    "secret-password",
		DisplayName: "Thiago",
	})
	want := []ValidationProblem{{Field: "username", Code: "invalid"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("problems mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateLoginInputNormalizesUsername(t *testing.T) {
	got := NormalizeLoginInput(LoginInput{
		Username: "  Thiago_Dev  ",
		Password: "secret-password",
	})
	want := LoginInput{
		Username: "thiago_dev",
		Password: "secret-password",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("login input mismatch (-want +got):\n%s", diff)
	}
}
