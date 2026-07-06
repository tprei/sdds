package note

const ListDefaultLimit = 50

type ListInput struct {
	CategorySlug CategorySlug
	Limit        int
}

func NormalizeListInput(input ListInput) ListInput {
	limit := input.Limit
	if limit == 0 {
		limit = ListDefaultLimit
	}

	return ListInput{
		CategorySlug: NormalizeCategorySlug(input.CategorySlug),
		Limit:        limit,
	}
}

func ValidateListInput(input ListInput) []ValidationProblem {
	normalized := NormalizeListInput(input)
	problems := make([]ValidationProblem, 0, 1)
	return appendLimitValidationProblems(problems, normalized.Limit)
}

func appendLimitValidationProblems(problems []ValidationProblem, limit int) []ValidationProblem {
	if limit < 1 {
		return append(problems, ValidationProblem{Field: "limit", Message: "required"})
	}
	return problems
}
