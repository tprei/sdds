package user

import (
	"strings"
	"unicode/utf8"
)

const (
	UsernameMinLength    = 3
	UsernameMaxLength    = 32
	DisplayNameMinLength = 1
	DisplayNameMaxLength = 60
	PasswordMinLength    = 8
	PasswordMaxLength    = 128
)

type ValidationProblem struct {
	Field string
	Code  string
}

func NormalizeCreateUserInput(input CreateUserInput) CreateUserInput {
	return CreateUserInput{
		Username:    NormalizeUsername(input.Username),
		Password:    input.Password,
		DisplayName: NormalizeDisplayName(input.DisplayName),
	}
}

func NormalizeLoginInput(input LoginInput) LoginInput {
	return LoginInput{
		Username: NormalizeUsername(input.Username),
		Password: input.Password,
	}
}

func NormalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizeDisplayName(value string) string {
	return strings.TrimSpace(value)
}

func ValidateCreateUserInput(input CreateUserInput) []ValidationProblem {
	normalized := NormalizeCreateUserInput(input)
	problems := make([]ValidationProblem, 0, 3)
	problems = appendUsernameValidationProblems(problems, normalized.Username)
	problems = appendPasswordValidationProblems(problems, normalized.Password)
	problems = appendDisplayNameValidationProblems(problems, normalized.DisplayName)
	return problems
}

func ValidateLoginInput(input LoginInput) []ValidationProblem {
	normalized := NormalizeLoginInput(input)
	problems := make([]ValidationProblem, 0, 2)
	problems = appendUsernameValidationProblems(problems, normalized.Username)
	problems = appendPasswordValidationProblems(problems, normalized.Password)
	return problems
}

func appendUsernameValidationProblems(problems []ValidationProblem, username string) []ValidationProblem {
	usernameLength := utf8.RuneCountInString(username)
	if usernameLength == 0 {
		return append(problems, ValidationProblem{Field: "username", Code: "required"})
	}
	if usernameLength < UsernameMinLength {
		return append(problems, ValidationProblem{Field: "username", Code: "too_short"})
	}
	if usernameLength > UsernameMaxLength {
		return append(problems, ValidationProblem{Field: "username", Code: "too_long"})
	}
	if !isUsernameIdentifier(username) {
		return append(problems, ValidationProblem{Field: "username", Code: "invalid"})
	}
	return problems
}

func appendDisplayNameValidationProblems(problems []ValidationProblem, displayName string) []ValidationProblem {
	displayNameLength := utf8.RuneCountInString(displayName)
	if displayNameLength == 0 {
		return append(problems, ValidationProblem{Field: "display_name", Code: "required"})
	}
	if displayNameLength > DisplayNameMaxLength {
		return append(problems, ValidationProblem{Field: "display_name", Code: "too_long"})
	}
	return problems
}

func appendPasswordValidationProblems(problems []ValidationProblem, password string) []ValidationProblem {
	passwordLength := utf8.RuneCountInString(password)
	if passwordLength == 0 {
		return append(problems, ValidationProblem{Field: "password", Code: "required"})
	}
	if passwordLength < PasswordMinLength {
		return append(problems, ValidationProblem{Field: "password", Code: "too_short"})
	}
	if passwordLength > PasswordMaxLength {
		return append(problems, ValidationProblem{Field: "password", Code: "too_long"})
	}
	return problems
}

func isUsernameIdentifier(value string) bool {
	for _, current := range value {
		if current >= 'a' && current <= 'z' {
			continue
		}
		if current >= '0' && current <= '9' {
			continue
		}
		switch current {
		case '_', '.', '-':
			continue
		default:
			return false
		}
	}
	return true
}
