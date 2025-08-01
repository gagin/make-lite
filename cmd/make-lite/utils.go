// cmd/make-lite/utils.go
package main

import (
	"strings"
)

// trimQuotes strips a single pair of matching quotes from the start and end of a string.
// It handles both single (') and double (") quotes.
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
		if s[0] == '\'' && s[len(s)-1] == '\'' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// cleanEnvLine processes a line from a .env file.
// It trims whitespace, ignores comments and blank lines, and splits into key/value.
// It returns the key, value, and a boolean indicating if the line was valid.
func cleanEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false // Invalid line format
	}

	// Per spec, "Anything preceding last token before assignment operator... is ignored."
	keyPart := strings.TrimSpace(parts[0])
	keyTokens := strings.Fields(keyPart)
	if len(keyTokens) == 0 {
		return "", "", false // Empty key
	}
	key := keyTokens[len(keyTokens)-1]

	val := strings.TrimSpace(parts[1])

	// Per spec, for .env files, strip surrounding quotes from the value.
	val = trimQuotes(val)

	return key, val, true
}
