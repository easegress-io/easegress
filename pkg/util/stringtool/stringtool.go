package stringtool

import "strings"

// Cat concatenates strings.
// It is intended to used in the core executing path for performance optimization.
// fmt.Printf is still recommended for readability.
func Cat(strs ...string) string {
	var builder strings.Builder
	for _, s := range strs {
		builder.WriteString(s)
	}

	return builder.String()
}
