package store

import (
	"encoding/base64"
	"strconv"
)

// encodeCursor renders an integer offset as an opaque base64 cursor token
// (endpoints.md §2: pagination cursors are opaque to clients).
func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

// decodeCursor parses an opaque cursor back into an offset; a missing or
// malformed cursor decodes to offset 0 (start of the list).
func decodeCursor(cursor string) int {
	if cursor == "" {
		return 0
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(string(raw))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
