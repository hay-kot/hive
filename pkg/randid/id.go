// Package randid provides random ID generation utilities.
package randid

import "math/rand/v2"

// Generate creates a random alphanumeric ID of the specified length.
func Generate(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}
	return string(b)
}
