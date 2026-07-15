package note

import (
	"strings"
	"unicode/utf8"
)

const (
	TitleMinLength           = 3
	TitleMaxLength           = 120
	BodyMaxLength            = 4000
	ClientRequestIDMaxLength = 128
	MaxImagesPerNote         = 1
)

type ValidationProblem struct {
	Field   string
	Message string
}

func NormalizeCreateInput(input CreateInput) CreateInput {
	return CreateInput{
		UserID:          input.UserID,
		Title:           strings.TrimSpace(input.Title),
		Body:            strings.TrimSpace(input.Body),
		CategorySlug:    NormalizeCategorySlug(input.CategorySlug),
		PlaceSlug:       NormalizePlaceSlug(input.PlaceSlug),
		ClientRequestID: input.ClientRequestID,
		ImageUploadIDs:  input.ImageUploadIDs,
	}
}

func ValidateCreateInput(input CreateInput) []ValidationProblem {
	normalized := NormalizeCreateInput(input)
	problems := make([]ValidationProblem, 0, 6)
	problems = appendTitleValidationProblems(problems, normalized.Title)
	problems = appendBodyValidationProblems(problems, normalized.Body)
	problems = appendCategoryValidationProblems(problems, normalized.CategorySlug)
	problems = appendClientRequestIDValidationProblems(problems, input.ClientRequestID)
	problems = appendImageUploadIDValidationProblems(problems, input.ImageUploadIDs)
	return problems
}
func appendClientRequestIDValidationProblems(problems []ValidationProblem, clientRequestID string) []ValidationProblem {
	clientRequestIDLength := utf8.RuneCountInString(clientRequestID)
	if clientRequestIDLength == 0 {
		return append(problems, ValidationProblem{Field: "client_request_id", Message: "required"})
	}
	if clientRequestIDLength > ClientRequestIDMaxLength {
		return append(problems, ValidationProblem{Field: "client_request_id", Message: "too_long"})
	}
	return problems
}

func appendImageUploadIDValidationProblems(problems []ValidationProblem, imageUploadIDs []string) []ValidationProblem {
	if len(imageUploadIDs) > MaxImagesPerNote {
		problems = append(problems, ValidationProblem{Field: "image_upload_ids", Message: "too_long"})
	}
	seen := make(map[string]struct{}, len(imageUploadIDs))
	for _, imageUploadID := range imageUploadIDs {
		if imageUploadID == "" {
			problems = append(problems, ValidationProblem{Field: "image_upload_ids", Message: "invalid"})
			continue
		}
		if _, ok := seen[imageUploadID]; ok {
			problems = append(problems, ValidationProblem{Field: "image_upload_ids", Message: "invalid"})
			continue
		}
		seen[imageUploadID] = struct{}{}
	}
	return problems
}

func appendTitleValidationProblems(problems []ValidationProblem, title string) []ValidationProblem {
	titleLength := utf8.RuneCountInString(title)
	if titleLength == 0 {
		return append(problems, ValidationProblem{Field: "title", Message: "required"})
	}
	if titleLength < TitleMinLength {
		return append(problems, ValidationProblem{Field: "title", Message: "too_short"})
	}
	if titleLength > TitleMaxLength {
		return append(problems, ValidationProblem{Field: "title", Message: "too_long"})
	}
	return problems
}

func appendBodyValidationProblems(problems []ValidationProblem, body string) []ValidationProblem {
	bodyLength := utf8.RuneCountInString(body)
	if bodyLength == 0 {
		return append(problems, ValidationProblem{Field: "body", Message: "required"})
	}
	if bodyLength > BodyMaxLength {
		return append(problems, ValidationProblem{Field: "body", Message: "too_long"})
	}
	return problems
}

func appendCategoryValidationProblems(problems []ValidationProblem, slug CategorySlug) []ValidationProblem {
	if slug == "" {
		return append(problems, ValidationProblem{Field: "category_slug", Message: "required"})
	}
	return problems
}
