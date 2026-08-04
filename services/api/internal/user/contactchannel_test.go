package user

import (
	"strings"
	"testing"
)

func TestNormalizeEmailLowercasesTrimsAndPreservesTagsAndDots(t *testing.T) {
	got := NormalizeEmail(" Ana.Silva+Notas@Example.COM ")
	want := "ana.silva+notas@example.com"
	if got != want {
		t.Fatalf("NormalizeEmail() = %q, want %q", got, want)
	}
}

func TestValidateEmail(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		wantField string
		wantCode  string
		wantOK    bool
	}{
		{name: "empty", value: "", wantField: "email", wantCode: "required"},
		{name: "no at", value: "notanemail", wantField: "email", wantCode: "invalid"},
		{name: "leading at", value: "@example.com", wantField: "email", wantCode: "invalid"},
		{name: "trailing at", value: "ana@", wantField: "email", wantCode: "invalid"},
		{name: "embedded space", value: "ana silva@example.com", wantField: "email", wantCode: "invalid"},
		{name: "two at signs", value: "a@b@c", wantField: "email", wantCode: "invalid"},
		{name: "embedded tab", value: "a\tb@c.com", wantField: "email", wantCode: "invalid"},
		{name: "trailing newline", value: "a@c.com\n", wantField: "email", wantCode: "invalid"},
		{name: "leading at only", value: "@c.com", wantField: "email", wantCode: "invalid"},
		{name: "valid plain", value: "ana@example.com", wantOK: true},
		{name: "valid with plus tag", value: "ana.silva+notas@example.com", wantOK: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems := ValidateEmail(tc.value)
			if tc.wantOK {
				if len(problems) != 0 {
					t.Fatalf("ValidateEmail(%q) = %v, want no problems", tc.value, problems)
				}
				return
			}
			if len(problems) != 1 {
				t.Fatalf("ValidateEmail(%q) = %v, want one problem", tc.value, problems)
			}
			if problems[0].Field != tc.wantField || problems[0].Code != tc.wantCode {
				t.Fatalf("ValidateEmail(%q) problem = {%s,%s}, want {%s,%s}", tc.value, problems[0].Field, problems[0].Code, tc.wantField, tc.wantCode)
			}
		})
	}
}

func TestValidateEmailRejectsTooLong(t *testing.T) {
	tooLong := strings.Repeat("a", EmailMaxLength+1)
	problems := ValidateEmail(tooLong)
	if len(problems) != 1 || problems[0].Code != "too_long" {
		t.Fatalf("ValidateEmail(tooLong) = %v, want one too_long problem", problems)
	}
}
