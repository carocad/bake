package functional

import (
	"github.com/agext/levenshtein"
)

func Map[T any, U any](slice []T, f func(T) U) []U {
	result := make([]U, len(slice))
	for i, t := range slice {
		result[i] = f(t)
	}
	return result
}

func Values[T any, U comparable](m map[U]T) []T {
	result := make([]T, len(m))
	for _, t := range m {
		result = append(result, t)
	}

	return result
}

func Suggest(text string, options []string) string {
	suggestion := ""
	bestDistance := len(text)
	for _, option := range options {
		dist := levenshtein.Distance(text, option, nil)
		if dist < bestDistance {
			suggestion = option
			bestDistance = dist
		}
	}

	return suggestion
}
