package note

import (
	"context"
	"errors"
)

var ErrCategoryNotFound = errors.New("category not found")
var ErrPlaceNotFound = errors.New("place not found")

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
		case "place_slug":
			errs = append(errs, ErrPlaceNotFound)
		}
	}
	return errs
}

func (err *CatalogValidationError) ValidationProblems() []ValidationProblem {
	problems := make([]ValidationProblem, 0, len(err.Problems))
	for _, field := range []string{"category_slug", "place_slug"} {
		for _, problem := range err.Problems {
			if problem.Field == field {
				problems = append(problems, problem)
			}
		}
	}
	for _, problem := range err.Problems {
		if problem.Field != "category_slug" && problem.Field != "place_slug" {
			problems = append(problems, problem)
		}
	}
	return problems
}

type Catalog interface {
	ListCategories(ctx context.Context) ([]Category, error)
	ListPlaces(ctx context.Context) ([]Place, error)
	FindActiveCategory(ctx context.Context, slug CategorySlug) (Category, error)
	FindActivePlace(ctx context.Context, slug PlaceSlug) (Place, error)
}
