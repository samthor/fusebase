package local

import (
	"strings"
)

const (
	invalidChars = ".$#[]/"
)

// ValidKey returns whether this individual path segment is valid.
func ValidKey(key string) bool {
	if len(key) == 0 {
		return false
	}
	for _, r := range key {
		if r <= 31 || r == 127 || strings.ContainsRune(invalidChars, r) {
			return false
		}
	}
	return true
}
