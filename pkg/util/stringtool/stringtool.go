package stringtool

import "strings"

// Cat concatenates strings.
// It is intended to used in the core executing path for performance optimization.
// fmt.Printf is still recommended for readability.
func Cat(strs ...string) string {
	n := 0
	for _, s := range strs {
		n += len(s)
	}

	var builder strings.Builder
	builder.Grow(n)
	for _, s := range strs {
		builder.WriteString(s)
	}

	return builder.String()
}

// StrInSlice returns whether the string is in the slice.
func StrInSlice(str string, slice []string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}

	return false
}
