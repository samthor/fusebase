package fusebase

import (
	"strconv"
	"strings"
)

// bytesToValue converts the written bytes to a native type to write to Firebase, incorporating
// the node's previous value (if any).
func bytesToValue(b []byte, prev interface{}) interface{} {
	if v, ok := prev.(string); ok && v != "" {
		return string(b) // previously a string, assume string
	}

	s := string(b)
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64) // need to TrimSpace for newline
	if err != nil {
		return s // string (nb. echo generates newlines, most people don't want them)
	}
	if _, ok := prev.(bool); ok {
		return f != 0 // prev was bool, assume number means bool again
	}
	return f // number
}
