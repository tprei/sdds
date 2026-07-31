package note

import (
	"context"
	"errors"
)

var ErrCategoryNotFound = errors.New("category not found")

type CatalogValidationError struct {
	Problems []ValidationProblem
}

func (err *CatalogValidationError) Error() string {
	return "catalog validation failed"
}

func (err *CatalogValidationError) Unwrap() []error {
	errs := make([]error, 0, len(err.Problems))
	for _, problem := range err.Problems {
		switch problem.Field {
		case "category_slug":
			errs = append(errs, ErrCategoryNotFound)
		}
	}
	return errs
}

func (err *CatalogValidationError) ValidationProblems() []ValidationProblem {
	problems := make([]ValidationProblem, 0, len(err.Problems))
	for _, problem := range err.Problems {
		if problem.Field == "category_slug" {
			problems = append(problems, problem)
		}
	}
	for _, problem := range err.Problems {
		if problem.Field != "category_slug" {
			problems = append(problems, problem)
		}
	}
	return problems
}

type Catalog interface {
	ListCategories(ctx context.Context) ([]Category, error)
	FindActiveCategory(ctx context.Context, slug CategorySlug) (Category, error)
}
